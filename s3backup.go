// This code is under BSD license. See license-bsd.txt
package main

/*
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
	"archive/zip"
	"crypto/sha1"
	"fmt"
	"io"
	"launchpad.net/goamz/aws"
	"launchpad.net/goamz/s3"
	"log"
	_ "mime"
	"os"
	"path/filepath"
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
	fmt.Printf("s3 bucket ok!\n")
}

// the names of files inside the zip file are relatitve to dirToZip e.g.
// if dirToZip is foo and there is a file foo/bar.txt, the name in the zip
// will be bar.txt
func createZipWithDirContent(zipFilePath, dirToZip string) error {
	if isDir, err := PathIsDir(dirToZip); err != nil || !isDir {
		// TODO: should return an error if err == nil && !isDir
		return err
	}
	zf, err := os.Create(zipFilePath)
	if err != nil {
		fmt.Printf("Failed to os.Create() %s, %s\n", zipFilePath, err.Error())
		return err
	}
	defer zf.Close()
	zipWriter := zip.NewWriter(zf)
	// TODO: is the order of defer here can create problems?
	// TODO: need to check error code returned by Close()
	defer zipWriter.Close()

	//fmt.Printf("Walk root: %s\n", config.LocalDir)
	err = filepath.Walk(dirToZip, func(pathToZip string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("WalkFunc() received err %s from filepath.Wath()\n", err.Error())
			return err
		}
		//fmt.Printf("%s\n", path)
		isDir, err := PathIsDir(pathToZip)
		if err != nil {
			fmt.Printf("PathIsDir() for %s failed with %s\n", pathToZip, err.Error())
			return err
		}
		if isDir {
			return nil
		}
		toZipReader, err := os.Open(pathToZip)
		if err != nil {
			fmt.Printf("os.Open() %s failed with %s\n", pathToZip, err.Error())
			return err
		}
		defer toZipReader.Close()

		zipName := pathToZip[len(dirToZip)+1:] // +1 for '/' in the path
		inZipWriter, err := zipWriter.Create(zipName)
		if err != nil {
			fmt.Printf("Error in zipWriter(): %s\n", err.Error())
			return err
		}
		_, err = io.Copy(inZipWriter, toZipReader)
		if err != nil {
			return err
		}
		fmt.Printf("Added %s to zip file\n", pathToZip)
		return nil
	})
	return nil
}

func fileSha1(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		//fmt.Printf("os.Open(%s) failed with %s\n", path, err.Error())
		return "", err
	}
	defer f.Close()
	h := sha1.New()
	_, err = io.Copy(h, f)
	if err != nil {
		//fmt.Printf("io.Copy() failed with %s\n", err.Error())
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// TODO: what to do about failures? Log somewhere and allow viewing via website?
// e-mail the failures once a day?
// TODO:
//  - upload the zip under YYMMDD_HHMM_${SHA1}.zip name
//    - but only if latest backup had a different ${SHA1}
func doBackup(config *BackupConfig) {
	// TODO: a better way to generate a random file name
	path := filepath.Join(os.TempDir(), "apptranslator-tmp-backup.zip")
	fmt.Printf("zip file name: %s\n", path)
	// TODO: do I need os.Remove() won't os.Create() over-write the file anyway?
	os.Remove(path) // remove before trying to create a new one, just in cased
	err := createZipWithDirContent(path, config.LocalDir)
	//defer os.Remove(path)
	if err != nil {
		return
	}
	sha1, err := fileSha1(path)
	if err != nil {
		return
	}
	fmt.Printf("%s  %s\n", sha1, path)
}

func BackupLoop(config *BackupConfig) {
	ensureValidConfig(config)
	doBackup(config)
	log.Fatalf("Exiting now")
	for {
		// sleep first so that we don't backup right after new deploy
		time.Sleep(backupFreq)
		fmt.Printf("Doing backup to s3\n")
		//b := s3.New(auth, region).Bucket(bucket)
	}
}
