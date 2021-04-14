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
	"strings"
	"sync"
	"time"

	"github.com/cloudfoundry-community/go-cfenv"
	"github.com/swisscom/backman/log"
	"github.com/swisscom/backman/s3"
	"github.com/swisscom/backman/service/util"
	"github.com/swisscom/backman/state"
)

var mysqlMutex = &sync.Mutex{}

func Backup(ctx context.Context, s3 *s3.Client, service util.Service, binding *cfenv.Service, filename string) error {
	state.BackupQueue(service)

	// lock global mysql mutex, only 1 backup/restore operation of this service-type is allowed to run in parallel
	// to avoid issues with setting MYSQL* environment variables and memory consumption
	mysqlMutex.Lock()
	defer mysqlMutex.Unlock()

	state.BackupStart(service)

	credentials := GetCredentials(binding)
	os.Setenv("MYSQL_PWD", credentials.Password)

	// prepare mysqldump command
	var command []string
	command = append(command, "mysqldump")
	command = append(command, "--single-transaction")
	command = append(command, "--quick")
	command = append(command, "--skip-add-locks")
	// https://serverfault.com/questions/912162/mysqldump-throws-unknown-table-column-statistics-in-information-schema-1109
	if service.DisableColumnStatistics {
		command = append(command, "--column-statistics=0")
	}
	command = append(command, "--hex-blob=TRUE")
	command = append(command, "-h")
	command = append(command, credentials.Hostname)
	command = append(command, "-P")
	command = append(command, credentials.Port)
	command = append(command, "-u")
	command = append(command, credentials.Username)
	if len(credentials.Database) > 0 {
		command = append(command, "--no-create-db")
		command = append(command, credentials.Database)
	} else {
		command = append(command, "--all-databases")
	}

	command = append(command, service.BackupOptions...)


	log.Debugf("executing mysql backup command: %v", strings.Join(command, " "))
	mysqldumpCmd := exec.CommandContext(ctx, command[0], command[1:]...)
    sedCmd := exec.CommandContext(ctx, "sed", "-e", "s/DEFINER=[^*]*\\*/\\*/g")

	// capture stdout to pass to gzipping buffer
	outPipe, _ := sedCmd.StdoutPipe()
	_, err := mysqldumpCmd.StdoutPipe()
	if err != nil {
		log.Errorf("could not get stdout pipe for mysqldump: %v", err)
		state.BackupFailure(service)
		return err
	}
	defer outPipe.Close()

    // pipe mysqldumpCmd to sedCmd
    sedPipeR, sedPipeW := io.Pipe()
    mysqldumpCmd.Stdout = sedPipeW
    sedCmd.Stdin = sedPipeR

	var uploadWait sync.WaitGroup
	uploadCtx, uploadCancel := context.WithCancel(context.Background()) // allows upload to be cancelable, in case backup times out
	defer uploadCancel()                                                // cancel upload in case Backup() exits before uploadWait is done

	// start upload in background, streaming output onto S3
	uploadWait.Add(1)
	go func() {
		defer uploadWait.Done()

		// gzipping stdout
		pr, pw := io.Pipe()
		gw := gzip.NewWriter(pw)
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

		objectPath := fmt.Sprintf("%s/%s/%s", service.Label, service.Name, filename)
		err = s3.UploadWithContext(uploadCtx, objectPath, pr, -1)
		if err != nil {
			log.Errorf("could not upload service backup [%s] to S3: %v", service.Name, err)
			state.BackupFailure(service)
		}
	}()
	time.Sleep(2 * time.Second) // wait for upload goroutine to be ready

	// capture and read stderr in case an error occurs
	var errBuf bytes.Buffer
	mysqldumpCmd.Stderr = &errBuf

	if err := mysqldumpCmd.Start(); err != nil {
		log.Errorf("could not run mysqldump: %v", err)
		state.BackupFailure(service)
		return err
	}
	sedCmd.Start()

	if err := mysqldumpCmd.Wait(); err != nil {
		state.BackupFailure(service)
		// check for timeout error
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("mysqldump: timeout: %v", ctx.Err())
		}

		log.Errorln(strings.TrimRight(errBuf.String(), "\r\n"))
		return fmt.Errorf("mysqldump: %v", err)
	}
	sedPipeW.Close()
    sedCmd.Wait()
	uploadWait.Wait() // wait for upload to have finished
	if err == nil {
		state.BackupSuccess(service)
	}
	return err
}
