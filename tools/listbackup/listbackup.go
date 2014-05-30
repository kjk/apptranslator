// This code is under BSD license. See license-bsd.txt
package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"strings"

	"github.com/crowdmob/goamz/aws"
	"github.com/crowdmob/goamz/s3"
	"github.com/garyburd/go-oauth/oauth"
)

var (
	bucketDelim = "/"

	oauthClient = oauth.Client{
		TemporaryCredentialRequestURI: "https://api.twitter.com/oauth/request_token",
		ResourceOwnerAuthorizationURI: "https://api.twitter.com/oauth/authenticate",
		TokenRequestURI:               "https://api.twitter.com/oauth/access_token",
	}

	config = struct {
		TwitterOAuthCredentials *oauth.Credentials
		Apps                    []AppConfig
		CookieAuthKeyHexStr     *string
		CookieEncrKeyHexStr     *string
		AwsAccess               *string
		AwsSecret               *string
		S3BackupBucket          *string
		S3BackupDir             *string
	}{
		&oauthClient.Credentials,
		nil,
		nil, nil,
		nil, nil,
		nil, nil,
	}
)

// a static configuration of a single app
type AppConfig struct {
	Name string
	// url for the application's website (shown in the UI)
	Url     string
	DataDir string
	// we authenticate only with Twitter, this is the twitter user name
	// of the admin user
	AdminTwitterUser string
	// an arbitrary string, used to protect the API for uploading new strings
	// for the app
	UploadSecret string
}

// reads the configuration file from the path specified by
// the config command line flag.
func readConfig(configFile string) error {
	b, err := ioutil.ReadFile(configFile)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, &config)
}

func fullUrl(bucket string) string {
	return fmt.Sprintf("http://%s.s3.amazonaws.com/", bucket)
}

// removes "/" if exists and adds delim if missing
func sanitizeDirForList(dir, delim string) string {
	if strings.HasPrefix(dir, "/") {
		dir = dir[1:]
	}
	if !strings.HasSuffix(dir, delim) {
		dir = dir + delim
	}
	return dir
}

func listBackups() {
	bucketName := *config.S3BackupBucket
	dir := sanitizeDirForList(*config.S3BackupDir, bucketDelim)
	auth := aws.Auth{AccessKey: *config.AwsAccess, SecretKey: *config.AwsSecret}
	b := s3.New(auth, aws.USEast).Bucket(bucketName)
	fmt.Printf("Listing files in %s\n", fullUrl(bucketName))
	rsp, err := b.List(dir, bucketDelim, "", 1000)
	if err != nil {
		log.Fatalf("Invalid s3 backup: bucket.List failed %s\n", err.Error())
	}
	//fmt.Printf("rsp: %v\n", rsp)
	if 0 == len(rsp.Contents) {
		fmt.Printf("There are no files in %s\n", fullUrl(*config.S3BackupBucket))
		return
	}
	//fmt.Printf("Backup files in %s:\n", fullUrl(*config.S3BackupBucket))
	for _, key := range rsp.Contents {
		fmt.Printf("  %s %d\n", key.Key, key.Size)
	}
}

func main() {
	readConfig("config.json")
	listBackups()
}
