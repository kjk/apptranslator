// This code is under BSD license. See license-bsd.txt
package main

/*
import (
	"os"
	"mime"
)

func upload(bucket s3.Bucket, local, remote string, public bool) error {
	localf, err := os.Open(local)
	if err != nil {
		return err
	}
	defer localf.Close()
	localfi, err := localf.Stat()
	if err != nil {
		return err
	}

	auth, region, err := readConfig()
	if err != nil {
		return err
	}

	var bucket, name string
	if i := strings.Index(remote, "/"); i >= 0 {
		bucket, name = remote[:i], remote[i+1:]
		if name == "" || strings.HasSuffix(name, "/") {
			name += path.Base(local)
		}
	} else {
		bucket = remote
		name = path.Base(local)
	}

	acl := s3.Private
	if public {
		acl = s3.PublicRead
	}

	contType := mime.TypeByExtension(path.Ext(local))
	if contType == "" {
		contType = "binary/octet-stream"
	}

	err = b.PutBucket(acl)
	if err != nil {
		return err
	}
	return b.PutReader(name, localf, localfi.Size(), contType, acl)
}
*/

import (
	"fmt"
	"launchpad.net/goamz/aws"
	"launchpad.net/goamz/s3"
	"log"
	_ "mime"
	"strings"
	"time"
)

var backupFreq = 4 * time.Hour
var bucketDelim = "/"

type BackupConfig struct {
	AwsAccess string
	AwsSecret string
	Bucket    string
	S3Dir     string
	LocalDir  string
}

func ensureValidConfig(config *BackupConfig) {
	if !PathExists(config.LocalDir) {
		log.Fatalf("Invalid s3 backup: directory to backup '%s' doesn't exist\n", config.LocalDir)
	}

	if !strings.HasSuffix(config.S3Dir, bucketDelim) {
		config.S3Dir += bucketDelim
	}

	auth := aws.Auth{config.AwsAccess, config.AwsSecret}
	s3 := s3.New(auth, aws.USEast)
	bucket := s3.Bucket(config.Bucket)
	_, err := bucket.List(config.S3Dir, bucketDelim, "", 10)
	if err != nil {
		log.Fatalf("Invalid s3 backup: bucket.List failed %s\n", err.Error())
	}
}

func BackupLoop(config *BackupConfig) {
	ensureValidConfig(config)
	for {
		// sleep first so that we don't backup right after new deploy
		time.Sleep(backupFreq)
		fmt.Printf("Doing backup to s3\n")
		//b := s3.New(auth, region).Bucket(bucket)
	}
}
