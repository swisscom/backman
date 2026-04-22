package mysql

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/swisscom/backman/config"
	"github.com/swisscom/backman/log"
	"github.com/swisscom/backman/s3"
	"github.com/swisscom/backman/state"
)

var mysqlMutex = &sync.Mutex{}

func Backup(ctx context.Context, s3 *s3.Client, service config.Service, filename string) error {
	state.BackupQueue(service)

	// lock global mysql mutex, only 1 backup/restore operation of this service-type is allowed to run in parallel
	// to avoid issues with setting MYSQL* environment variables and memory consumption
	mysqlMutex.Lock()
	defer mysqlMutex.Unlock()

	state.BackupStart(service, filename)

	os.Setenv("MYSQL_PWD", service.Binding.Password)

	// prepare mysqldump command
	var command []string
	command = append(command, "mysqldump")
	command = append(command, "--single-transaction")
	command = append(command, "--quick")
	command = append(command, "--skip-add-locks")
	// https://serverfault.com/questions/912162/mysqldump-throws-unknown-table-column-statistics-in-information-schema-1109
	if config.Get().DisableAllColumnStatistics || service.DisableColumnStatistics {
		command = append(command, "--column-statistics=0")
	}
	command = append(command, "-h")
	command = append(command, service.Binding.Host)
	command = append(command, "-P")
	command = append(command, strconv.Itoa(service.Binding.Port))
	command = append(command, "-u")
	command = append(command, service.Binding.Username)

	// ssl/tls
	if len(service.Binding.SSL.ClientCertPath) > 0 {
		command = append(command, "--ssl-cert="+service.Binding.SSL.ClientCertPath)
	}

	if len(service.Binding.SSL.CACertPath) > 0 {
		command = append(command, "--ssl-ca="+service.Binding.SSL.CACertPath)
	}

	if len(service.Binding.SSL.ClientKeyPath) > 0 {
		command = append(command, "--ssl-key="+service.Binding.SSL.ClientKeyPath)
	}

	if service.Binding.SSL.VerifyServerCert {
		command = append(command, "--ssl-verify-server-cert")
	}

	if len(service.Binding.Database) > 0 {
		command = append(command, "--no-create-db")
		command = append(command, service.Binding.Database)
		for _, ignoreTable := range service.IgnoreTables {
			command = append(command, "--ignore-table="+service.Binding.Database+"."+ignoreTable)
		}
	} else {
		command = append(command, "--all-databases")
	}
	command = append(command, service.BackupOptions...)

	log.Debugf("executing mysql backup command: %v", strings.Join(command, " "))
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)

	// capture stdout to pass to gzipping buffer
	outPipe, err := cmd.StdoutPipe()
	if err != nil {
		log.Errorf("could not get stdout pipe for mysqldump: %v", err)
		state.BackupFailure(service, filename)
		return err
	}
	defer outPipe.Close()

	var uploadWait sync.WaitGroup
	uploadCtx, uploadCancel := context.WithCancel(ctx) // allows upload to be cancelable, in case backup times out
	defer uploadCancel()                               // cancel upload in case Backup() exits before uploadWait is done

	// start upload in background, streaming output onto S3
	uploadWait.Add(1)
	go func() {
		defer uploadWait.Done()

		// gzipping stdout, pass to gzipping buffer
		pr, pw := io.Pipe()
		defer pw.Close()

		gw := gzip.NewWriter(pw)
		defer gw.Close()
		defer gw.Flush()

		gw.Name = strings.TrimSuffix(filename, ".gz")
		gw.ModTime = time.Now()
		go func() {
			_, _ = io.Copy(gw, bufio.NewReader(outPipe))
			if err := gw.Flush(); err != nil {
				log.Errorf("%v", err)
			}
			if err := gw.Close(); err != nil {
				log.Errorf("%v", err)
			}
			if err := pw.Close(); err != nil {
				log.Errorf("%v", err)
			}
		}()

		objectPath := fmt.Sprintf("%s/%s/%s", service.Binding.Type, service.Name, filename)
		err = s3.UploadWithContext(uploadCtx, objectPath, pr, -1)
		if err != nil {
			log.Errorf("could not upload service backup [%s] to S3: %v", service.Name, err)
			state.BackupFailure(service, filename)
		}
		time.Sleep(1 * time.Second) // wait pipe to be closed
	}()
	time.Sleep(5 * time.Second) // wait for upload goroutine to be ready

	// capture and read stderr in case an error occurs
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf

	if err := cmd.Start(); err != nil {
		log.Errorf("could not run mysqldump: %v", err)
		state.BackupFailure(service, filename)
		return err
	}
	time.Sleep(5 * time.Second) // wait for upload goroutine to start working

	if err := cmd.Wait(); err != nil {
		state.BackupFailure(service, filename)
		// check for timeout error
		if ctx.Err() == context.DeadlineExceeded {
			if service.LogStdErr {
				log.Errorln(strings.TrimRight(errBuf.String(), "\r\n"))
			}
			return fmt.Errorf("mysqldump: timeout: %v", ctx.Err())
		}

		log.Errorln(strings.TrimRight(errBuf.String(), "\r\n"))
		return fmt.Errorf("mysqldump: %v", err)
	}
	time.Sleep(5 * time.Second) // wait for backup goroutine to have finished entirely

	uploadWait.Wait() // wait for upload to have finished
	if err == nil {
		state.BackupSuccess(service, filename)
	}

	if service.LogStdErr {
		log.Infoln(strings.TrimRight(errBuf.String(), "\r\n"))
	}
	return err
}
