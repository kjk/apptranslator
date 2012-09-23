// This code is under BSD license. See license-bsd.txt
package main

import (
	"fmt"
	"launchpad.net/goamz/aws"
	"launchpad.net/goamz/s3"
	"log"
	"encoding/json"
	"github.com/garyburd/go-oauth/oauth"
	"io/ioutil"
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

// readSecrets reads the configuration file from the path specified by
// the config command line flag.
func readSecrets(configFile string) error {
	b, err := ioutil.ReadFile(configFile)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, &config)
}
/*
// The ListResp type holds the results of a List bucket operation.
type ListResp struct {
	Name      string
	Prefix    string
	Delimiter string
	Marker    string
	MaxKeys   int
	// IsTruncated is true if the results have been truncated because
	// there are more keys and prefixes than can fit in MaxKeys.
	// N.B. this is the opposite sense to that documented (incorrectly) in
	// http://goo.gl/YjQTc
	IsTruncated    bool
	Contents       []Key
	CommonPrefixes []string `xml:">Prefix"`
}

// The Key type represents an item stored in an S3 bucket.
type Key struct {
	Key          string
	LastModified string
	Size         int64
	// ETag gives the hex-encoded MD5 sum of the contents,
	// surrounded with double-quotes.
	ETag         string
	StorageClass string
	Owner        Owner
}
*/

func fullUrl(bucket, path string) string {
	return fmt.Sprintf("http://%s.s3.amazonaws.com%s", bucket, path)
}

func listBackups() {
	auth := aws.Auth{*config.AwsAccess, *config.AwsSecret}
	s3 := s3.New(auth, aws.USEast)
	bucket := s3.Bucket(*config.S3BackupBucket)
	rsp, err := bucket.List(*config.S3BackupDir, bucketDelim, "", 1000)
	if err != nil {
		log.Fatalf("Invalid s3 backup: bucket.List failed %s\n", err.Error())
	}
	if 0 == len(rsp.Contents) {
		fmt.Printf("There are no files in %s\n", fullUrl(*config.S3BackupBucket, *config.S3BackupDir))
		return
	}
	fmt.Printf("Backup files in %s:\n", fullUrl(*config.S3BackupBucket, *config.S3BackupDir))
	for _, key := range rsp.Contents {
		fmt.Printf("  %s %d\n", key.Key, key.Size)
	}
}

func main() {
	readSecrets("secrets.json")
	listBackups()
}
