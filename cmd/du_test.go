package cmd

import (
	"coscli/util"
	"fmt"
	"testing"

	. "github.com/agiledragon/gomonkey/v2"
	. "github.com/smartystreets/goconvey/convey"
	"github.com/tencentyun/cos-go-sdk-v5"
)

func TestDuCmd(t *testing.T) {
	fmt.Println("TestDuCmd")
	testBucket = randStr(8)
	testAlias = testBucket + "-alias"
	testOfsBucket = randStr(8)
	testOfsBucketAlias = testOfsBucket + "-alias"
	testVersionBucket = randStr(8)
	testVersionBucketAlias = testVersionBucket + "-alias"
	setUp(testBucket, testAlias, testEndpoint, false, false)
	defer tearDown(testBucket, testAlias, testEndpoint, false)
	setUp(testOfsBucket, testOfsBucketAlias, testEndpoint, true, false)
	defer tearDown(testOfsBucket, testOfsBucketAlias, testEndpoint, false)
	setUp(testVersionBucket, testVersionBucketAlias, testEndpoint, false, true)
	//defer tearDown(testVersionBucket, testVersionBucketAlias, testEndpoint, true)
	clearCmd()
	cmd := rootCmd
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	genDir(testDir, 3)
	defer delDir(testDir)
	localFileName := fmt.Sprintf("%s/small-file", testDir)

	cosFileName := fmt.Sprintf("cos://%s/%s", testAlias, "multi-small")
	args := []string{"cp", localFileName, cosFileName, "-r"}
	cmd.SetArgs(args)
	cmd.Execute()

	ofsFileName := fmt.Sprintf("cos://%s/%s", testOfsBucketAlias, "multi-small")

	args = []string{"cp", localFileName, ofsFileName, "-r"}
	cmd.SetArgs(args)
	cmd.Execute()

	versioningFileName := fmt.Sprintf("cos://%s/%s", testVersionBucketAlias, "multi-small")
	args = []string{"cp", localFileName, versioningFileName, "-r"}
	cmd.SetArgs(args)
	cmd.Execute()
	Convey("Test coscli du", t, func() {
		Convey("success", func() {
			Convey("duBucket", func() {
				clearCmd()
				cmd := rootCmd
				args = []string{"du", fmt.Sprintf("cos://%s", testAlias)}
				cmd.SetArgs(args)
				e := cmd.Execute()
				So(e, ShouldBeNil)
			})
			Convey("duCosObjects", func() {
				clearCmd()
				cmd := rootCmd
				args = []string{"du", cosFileName}
				cmd.SetArgs(args)
				e := cmd.Execute()
				So(e, ShouldBeNil)
			})
			Convey("duOfsObjects", func() {
				clearCmd()
				cmd := rootCmd
				args = []string{"du", ofsFileName}
				cmd.SetArgs(args)
				e := cmd.Execute()
				So(e, ShouldBeNil)
			})
			Convey("duCosObjectVersions", func() {
				clearCmd()
				cmd := rootCmd
				args = []string{"du", versioningFileName, "--all-versions"}
				cmd.SetArgs(args)
				e := cmd.Execute()
				So(e, ShouldBeNil)
			})
		})
		Convey("fail", func() {
			Convey("not enough arguments", func() {
				clearCmd()
				cmd := rootCmd
				args = []string{"du"}
				cmd.SetArgs(args)
				e := cmd.Execute()
				fmt.Printf(" : %v", e)
				So(e, ShouldBeError)
			})
			Convey("FormatUrl", func() {
				patches := ApplyFunc(util.FormatUrl, func(urlStr string) (util.StorageUrl, error) {
					return nil, fmt.Errorf("test formaturl fail")
				})
				defer patches.Reset()
				clearCmd()
				cmd := rootCmd
				args = []string{"du", "invalid"}
				cmd.SetArgs(args)
				e := cmd.Execute()
				fmt.Printf(" : %v", e)
				So(e, ShouldBeError)
			})
			Convey("not cos url", func() {
				clearCmd()
				cmd := rootCmd
				args = []string{"du", "invalid"}
				cmd.SetArgs(args)
				e := cmd.Execute()
				fmt.Printf(" : %v", e)
				So(e, ShouldBeError)
			})
			Convey("NewClient", func() {
				patches := ApplyFunc(util.NewClient, func(config *util.Config, param *util.Param, bucketName string) (client *cos.Client, err error) {
					return nil, fmt.Errorf("test NewClient error")
				})
				defer patches.Reset()
				clearCmd()
				cmd := rootCmd
				args = []string{"du", fmt.Sprintf("cos://%s", testAlias)}
				cmd.SetArgs(args)
				e := cmd.Execute()
				fmt.Printf(" : %v", e)
				So(e, ShouldBeError)
			})
		})
	})
}
