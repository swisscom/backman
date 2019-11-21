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

	host, _ := binding.CredentialString("host")
	database, _ := binding.CredentialString("database")
	username, _ := binding.CredentialString("username")
	password, _ := binding.CredentialString("password")
	port, _ := binding.CredentialString("port")
	if len(port) == 0 {
		switch p := binding.Credentials["port"].(type) {
		case float64:
			port = strconv.Itoa(int(p))
		case int, int32, int64:
			port = strconv.Itoa(p.(int))
		}
	}

	os.Setenv("MYSQL_PWD", password)

	// prepare mysqldump command
	var command []string
	command = append(command, "sh")
	command = append(command, "-c")
	var command_string_array []string
	command_string_array = append(command_string_array, "mysqldump")
	command_string_array = append(command_string_array, "--single-transaction")
	command_string_array = append(command_string_array, "--quick")
	command_string_array = append(command_string_array, "--skip-add-locks")
	command_string_array = append(command_string_array, "-h")
	command_string_array = append(command_string_array, host)
	command_string_array = append(command_string_array, "-P")
	command_string_array = append(command_string_array, port)
	command_string_array = append(command_string_array, "-u")
	command_string_array = append(command_string_array, username)
	if len(database) > 0 {
		command_string_array = append(command_string_array, "--databases")
		command_string_array = append(command_string_array, database)
	} else {
		command_string_array = append(command_string_array, "--all-databases")
	}
	command_string_array = append(command_string_array, "| sed -e 's/DEFINER[ ]*=[ ]*[^*]*\\*/\\*/'")
	command = append(command, strings.Join(command_string_array, " "))

	log.Debugf("executing mysql backup command: %v", strings.Join(command, " "))
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)

	// capture stdout to pass to gzipping buffer
	outPipe, err := cmd.StdoutPipe()
	if err != nil {
		log.Errorf("could not get stdout pipe for mysqldump: %v", err)
		state.BackupFailure(service)
		return err
	}
	defer outPipe.Close()

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
		}
	}()
	time.Sleep(2 * time.Second) // wait for upload goroutine to be ready

	// capture and read stderr in case an error occurs
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf

	if err := cmd.Start(); err != nil {
		log.Errorf("could not run mysqldump: %v", err)
		state.BackupFailure(service)
		return err
	}

	if err := cmd.Wait(); err != nil {
		state.BackupFailure(service)
		// check for timeout error
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("mysqldump: timeout: %v", ctx.Err())
		}

		log.Errorln(strings.TrimRight(errBuf.String(), "\r\n"))
		return fmt.Errorf("mysqldump: %v", err)
	}

	uploadWait.Wait() // wait for upload to have finished
	if err == nil {
		state.BackupSuccess(service)
	}
	return err
}
