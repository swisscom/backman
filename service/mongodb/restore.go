package mongodb

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/cloudfoundry-community/go-cfenv"
	"github.com/swisscom/backman/log"
	"github.com/swisscom/backman/s3"
	"github.com/swisscom/backman/service/util"
	"github.com/swisscom/backman/state"
)

func Restore(ctx context.Context, s3 *s3.Client, service util.Service, binding *cfenv.Service, objectPath string) error {
	state.RestoreQueue(service)

	// lock global mongodb mutex, only 1 backup of this service-type is allowed to run in parallel
	mongoMutex.Lock()
	defer mongoMutex.Unlock()

	state.RestoreStart(service)

	uri, _ := binding.CredentialString("uri")
	database, _ := binding.CredentialString("database")

	var dropCommand []string

	dropCommand = append(dropCommand, "mongo")
	dropCommand = append(dropCommand, uri)
	dropCommand = append(dropCommand, "--eval")
	dropCommand = append(dropCommand, "\"db.getCollectionNames().forEach(function(col){db[col].drop()});\"")

	log.Debugf("executing mongodb collection drop command: %v", strings.Join(dropCommand, " "))
	dropCmd := exec.CommandContext(ctx, dropCommand[0], dropCommand[1:]...)

	// print out stdout/stderr
	dropCmd.Stdout = os.Stdout
	dropCmd.Stderr = os.Stderr

	if err := dropCmd.Start(); err != nil {
		log.Errorf("could not run mongo drop collections: %v", err)
		state.RestoreFailure(service)
		return err
	}

	if err := dropCmd.Wait(); err != nil {
		state.RestoreFailure(service)
		// check for timeout error
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("mongo drop collections: timeout: %v", ctx.Err())
		}
		return fmt.Errorf("mongo drop collections: %v", err)
	}

	// prepare mongorestore command
	var command []string
	command = append(command, "mongorestore")
	command = append(command, "--uri")
	command = append(command, "\""+uri+"\"")
	command = append(command, "--nsFrom")
	command = append(command, "\"$prefix$.$suffix$\"")
	command = append(command, "--nsTo")
	command = append(command, "\""+database+".$suffix$\"")
	command = append(command, "--numInsertionWorkersPerCollection")
	command = append(command, "16")
	command = append(command, "--gzip")
	command = append(command, "--archive")

	log.Debugf("executing mongodb restore command: %v", strings.Join(command, " "))
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)

	downloadCtx, downloadCancel := context.WithCancel(context.Background()) // allows download to be cancelable, in case restore times out
	defer downloadCancel()                                                  // cancel download in case Restore() exits before downloadWait is done

	// streaming from S3 for stdin
	reader, err := s3.DownloadWithContext(downloadCtx, objectPath)
	if err != nil {
		log.Errorf("could not download service backup [%s] from S3: %v", service.Name, err)
		state.RestoreFailure(service)
		return err
	}
	defer reader.Close()
	cmd.Stdin = bufio.NewReader(reader)

	// print out stdout/stderr
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		log.Errorf("could not run mongorestore: %v", err)
		state.RestoreFailure(service)
		return err
	}

	if err := cmd.Wait(); err != nil {
		state.RestoreFailure(service)
		// check for timeout error
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("mongorestore: timeout: %v", ctx.Err())
		}
		return fmt.Errorf("mongorestore: %v", err)
	}

	if err == nil {
		state.RestoreSuccess(service)
	}
	return err
}
