package util

import (
	"context"
	"encoding/xml"
	"fmt"
	logger "github.com/sirupsen/logrus"
	"github.com/tencentyun/cos-go-sdk-v5"
	"net/url"
)

var succeedNum, failedNum, errTypeNum int

func RestoreObject(c *cos.Client, bucketName, objectKey string, days int, mode string) error {
	opt := &cos.ObjectRestoreOptions{
		XMLName:       xml.Name{},
		Days:          days,
		Tier:          &cos.CASJobParameters{Tier: mode},
		XOptionHeader: nil,
	}

	logger.Infof("Restore cos://%s/%s\n", bucketName, objectKey)
	_, err := c.Object.PostRestore(context.Background(), objectKey, opt)
	if err != nil {
		return err
	}
	return nil
}

func RestoreObjects(c *cos.Client, cosUrl StorageUrl, fo *FileOperations) error {
	// 根据s.Header判断是否是融合桶或者普通桶
	s, err := c.Bucket.Head(context.Background())
	if err != nil {
		return err
	}
	logger.Infof("Start Restore %s", cosUrl.(*CosUrl).Bucket+cosUrl.(*CosUrl).Object)
	if s.Header.Get("X-Cos-Bucket-Arch") == "OFS" {
		bucketName := cosUrl.(*CosUrl).Bucket
		prefix := cosUrl.(*CosUrl).Object
		err = restoreOfsObjects(c, bucketName, prefix, fo, "")
	} else {
		err = restoreCosObjects(c, cosUrl, fo)
	}
	logger.Infof("Restore %s completed,total num: %d,success num: %d,restore error num: %d,error type num: %d", cosUrl.(*CosUrl).Bucket+cosUrl.(*CosUrl).Object, succeedNum+failedNum+errTypeNum, succeedNum, failedNum, errTypeNum)
	return nil
}

func restoreCosObjects(c *cos.Client, cosUrl StorageUrl, fo *FileOperations) error {
	var err error
	var objects []cos.Object
	marker := ""
	isTruncated := true

	for isTruncated {
		err, objects, _, isTruncated, marker = getCosObjectListForLs(c, cosUrl, marker, 0, true)
		if err != nil {
			return fmt.Errorf("list objects error : %v", err)
		}

		for _, object := range objects {
			if object.StorageClass == Archive || object.StorageClass == MAZArchive || object.StorageClass == DeepArchive {
				object.Key, _ = url.QueryUnescape(object.Key)
				if cosObjectMatchPatterns(object.Key, fo.Operation.Filters) {
					err := RestoreObject(c, cosUrl.(*CosUrl).Bucket, object.Key, fo.Operation.Days, fo.Operation.RestoreMode)
					if err != nil {
						failedNum += 1
						writeError(fmt.Sprintf("restore %s failed , errMsg:%v\n", object.Key, err), fo)
					} else {
						succeedNum += 1
					}
				}
			} else {
				errTypeNum += 1
				writeError(fmt.Sprintf("restore %s failed , errMsg:The file type is %s, and restore only supports Archive, MAZArchive, and DeepArchive three types.\n", object.Key, object.StorageClass), fo)
			}

		}
	}

	return nil
}

func restoreOfsObjects(c *cos.Client, bucketName, prefix string, fo *FileOperations, marker string) error {
	var err error
	var objects []cos.Object
	var commonPrefixes []string
	isTruncated := true

	for isTruncated {
		err, objects, commonPrefixes, isTruncated, marker = getOfsObjectListForLs(c, prefix, marker, 0, true)
		if err != nil {
			return fmt.Errorf("list objects error : %v", err)
		}

		for _, object := range objects {
			if object.StorageClass == Archive || object.StorageClass == MAZArchive || object.StorageClass == DeepArchive {
				object.Key, _ = url.QueryUnescape(object.Key)
				if cosObjectMatchPatterns(object.Key, fo.Operation.Filters) {
					err := RestoreObject(c, bucketName, object.Key, fo.Operation.Days, fo.Operation.RestoreMode)
					if err != nil {
						failedNum += 1
						writeError(fmt.Sprintf("restore %s failed , errMsg:%v\n", object.Key, err), fo)
					} else {
						succeedNum += 1
					}
				}
			} else {
				errTypeNum += 1
				writeError(fmt.Sprintf("restore %s failed , errMsg:The file type is %s, and restore only supports Archive, MAZArchive, and DeepArchive three types.\n", object.Key, object.StorageClass), fo)
			}
		}

		if len(commonPrefixes) > 0 {
			for _, commonPrefix := range commonPrefixes {
				commonPrefix, _ = url.QueryUnescape(commonPrefix)
				// 递归目录
				err = restoreOfsObjects(c, bucketName, commonPrefix, fo, "")
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}
