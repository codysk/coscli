package util

import (
	"context"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/olekukonko/tablewriter"

	"github.com/tencentyun/cos-go-sdk-v5"
)

func UrlDecodeCosPattern(objects []cos.Object) []cos.Object {
	res := make([]cos.Object, 0)
	for _, o := range objects {
		o.Key, _ = url.QueryUnescape(o.Key)
		res = append(res, o)
	}
	return res
}

func MatchCosPattern(objects []cos.Object, pattern string, include bool) []cos.Object {
	res := make([]cos.Object, 0)
	for _, o := range objects {
		match, _ := regexp.Match(pattern, []byte(o.Key))
		if !include {
			match = !match
		}
		if match {
			res = append(res, o)
		}
	}
	return res
}

func MatchUploadPattern(uploads []UploadInfo, pattern string, include bool) []UploadInfo {
	res := make([]UploadInfo, 0)
	for _, u := range uploads {
		match, _ := regexp.Match(pattern, []byte(u.Key))
		if !include {
			match = !match
		}
		if match {
			res = append(res, u)
		}
	}
	return res
}

func GetObjectsListRecursive(c *cos.Client, prefix string, limit int, include string, exclude string, retryCount ...int) (objects []cos.Object,
	commonPrefixes []string, err error) {

	opt := &cos.BucketGetOptions{
		Prefix:       prefix,
		Delimiter:    "",
		EncodingType: "url",
		Marker:       "",
		MaxKeys:      limit,
	}

	isTruncated := true
	marker := ""
	for isTruncated {
		opt.Marker = marker

		res, err := tryGetObjects(c, opt)
		if err != nil {
			return objects, commonPrefixes, err
		}

		objects = append(objects, res.Contents...)
		commonPrefixes = res.CommonPrefixes

		if limit > 0 {
			isTruncated = false
		} else {
			isTruncated = res.IsTruncated
			marker, _ = url.QueryUnescape(res.NextMarker)
		}
	}

	// 对key进行urlDecode解码
	objects = UrlDecodeCosPattern(objects)

	if len(include) > 0 {
		objects = MatchCosPattern(objects, include, true)
	}
	if len(exclude) > 0 {
		objects = MatchCosPattern(objects, exclude, false)
	}

	return objects, commonPrefixes, nil
}

// get objects限频重试(最多重试10次，每次重试间隔1-10s随机)
func tryGetObjects(c *cos.Client, opt *cos.BucketGetOptions) (*cos.BucketGetResult, error) {
	for i := 0; i <= 10; i++ {
		res, resp, err := c.Bucket.Get(context.Background(), opt)
		if err != nil {
			if resp != nil && resp.StatusCode == 503 {
				if i == 10 {
					return res, err
				} else {
					fmt.Println("Error 503: Service Unavailable. Retrying...")
					waitTime := time.Duration(rand.Intn(10)+1) * time.Second
					time.Sleep(waitTime)
					continue
				}
			} else {
				return res, err
			}
		} else {
			return res, err
		}
	}
	return nil, fmt.Errorf("Retry limit exceeded")
}

func tryGetObjectVersions(c *cos.Client, opt *cos.BucketGetObjectVersionsOptions) (*cos.BucketGetObjectVersionsResult, error) {
	for i := 0; i <= 10; i++ {
		res, resp, err := c.Bucket.GetObjectVersions(context.Background(), opt)
		if err != nil {
			if resp != nil && resp.StatusCode == 503 {
				if i == 10 {
					return res, err
				} else {
					fmt.Println("Error 503: Service Unavailable. Retrying...")
					waitTime := time.Duration(rand.Intn(10)+1) * time.Second
					time.Sleep(waitTime)
					continue
				}
			} else {
				return res, err
			}
		} else {
			return res, err
		}
	}
	return nil, fmt.Errorf("Retry limit exceeded")
}

func tryGetUploads(c *cos.Client, opt *cos.ListMultipartUploadsOptions) (*cos.ListMultipartUploadsResult, error) {
	for i := 0; i <= 10; i++ {
		res, resp, err := c.Bucket.ListMultipartUploads(context.Background(), opt)
		if err != nil {
			if resp != nil && resp.StatusCode == 503 {
				if i == 10 {
					return res, err
				} else {
					fmt.Println("Error 503: Service Unavailable. Retrying...")
					waitTime := time.Duration(rand.Intn(10)+1) * time.Second
					time.Sleep(waitTime)
					continue
				}
			} else {
				return res, err
			}
		} else {
			return res, err
		}
	}
	return nil, fmt.Errorf("Retry limit exceeded")
}

func tryGetParts(c *cos.Client, prefix, uploadId string, opt *cos.ObjectListPartsOptions) (*cos.ObjectListPartsResult, error) {
	for i := 0; i <= 10; i++ {
		res, resp, err := c.Object.ListParts(context.Background(), prefix, uploadId, opt)
		if err != nil {
			if resp != nil && resp.StatusCode == 503 {
				if i == 10 {
					return res, err
				} else {
					fmt.Println("Error 503: Service Unavailable. Retrying...")
					waitTime := time.Duration(rand.Intn(10)+1) * time.Second
					time.Sleep(waitTime)
					continue
				}
			} else {
				return res, err
			}
		} else {
			return res, err
		}
	}
	return nil, fmt.Errorf("Retry limit exceeded")
}

// =====new

func ListObjects(c *cos.Client, cosUrl StorageUrl, limit int, recursive bool, filters []FilterOptionType) error {
	var err error
	var objects []cos.Object
	var commonPrefixes []string
	total := 0
	isTruncated := true
	marker := ""

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Key", "Type", "Last Modified", "Etag", "Size", "RestoreStatus"})
	table.SetBorder(false)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetAutoWrapText(false)

	for isTruncated && total < limit {
		table.ClearRows()
		queryLimit := 1000
		if limit-total < 1000 {
			queryLimit = limit - total
		}

		err, objects, commonPrefixes, isTruncated, marker = getCosObjectListForLs(c, cosUrl, marker, queryLimit, recursive)

		if err != nil {
			return fmt.Errorf("list objects error : %v", err)
		}

		if len(commonPrefixes) > 0 {
			for _, commonPrefix := range commonPrefixes {
				if cosObjectMatchPatterns(commonPrefix, filters) {
					table.Append([]string{commonPrefix, "DIR", "", "", "", ""})
					total++
				}
			}
		}

		for _, object := range objects {
			object.Key, _ = url.QueryUnescape(object.Key)
			if cosObjectMatchPatterns(object.Key, filters) {
				utcTime, err := time.Parse(time.RFC3339, object.LastModified)
				if err != nil {
					return fmt.Errorf("Error parsing time:%v", err)
				}
				table.Append([]string{object.Key, object.StorageClass, utcTime.Local().Format(time.RFC3339), object.ETag, formatBytes(float64(object.Size)), object.RestoreStatus})
				total++
			}
		}

		if !isTruncated || total >= limit {
			table.SetFooter([]string{"", "", "", "", "Total Objects: ", fmt.Sprintf("%d", total)})
			table.Render()
			break
		}
		table.Render()

		// 重置表格
		table = tablewriter.NewWriter(os.Stdout)
		table.SetBorder(false)
		table.SetAlignment(tablewriter.ALIGN_LEFT)
		table.SetAutoWrapText(false)
	}

	return nil
}

func ListObjectVersions(c *cos.Client, cosUrl StorageUrl, limit int, recursive bool, filters []FilterOptionType) error {
	var err error
	var versions []cos.ListVersionsResultVersion
	var deleteMarkers []cos.ListVersionsResultDeleteMarker
	var commonPrefixes []string
	total := 0
	isTruncated := true

	var keyMarker, versionIdMarker string

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Key", "Type", "VersionId", "IsLatest", "Delete Marker", "Last Modified", "Etag", "Size"})
	table.SetBorder(false)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetAutoWrapText(false)

	for isTruncated && total < limit {
		table.ClearRows()
		queryLimit := 1000
		if limit-total < 1000 {
			queryLimit = limit - total
		}

		err, versions, deleteMarkers, commonPrefixes, isTruncated, versionIdMarker, keyMarker = getCosObjectVersionListForLs(c, cosUrl, versionIdMarker, keyMarker, queryLimit, recursive)

		if err != nil {
			return fmt.Errorf("list objects error : %v", err)
		}

		if len(commonPrefixes) > 0 {
			for _, commonPrefix := range commonPrefixes {
				if cosObjectMatchPatterns(commonPrefix, filters) {
					table.Append([]string{commonPrefix, "DIR", "", "", "", "", "", ""})
					total++
				}
			}
		}

		for _, object := range versions {
			object.Key, _ = url.QueryUnescape(object.Key)
			if cosObjectMatchPatterns(object.Key, filters) {
				utcTime, err := time.Parse(time.RFC3339, object.LastModified)
				if err != nil {
					return fmt.Errorf("Error parsing time:%v", err)
				}

				table.Append([]string{object.Key, object.StorageClass, object.VersionId, strconv.FormatBool(object.IsLatest), strconv.FormatBool(false), utcTime.Local().Format(time.RFC3339), object.ETag, formatBytes(float64(object.Size))})
				total++
			}
		}

		for _, object := range deleteMarkers {
			object.Key, _ = url.QueryUnescape(object.Key)
			if cosObjectMatchPatterns(object.Key, filters) {
				utcTime, err := time.Parse(time.RFC3339, object.LastModified)
				if err != nil {
					return fmt.Errorf("Error parsing time:%v", err)
				}
				table.Append([]string{object.Key, "", object.VersionId, strconv.FormatBool(object.IsLatest), strconv.FormatBool(true), utcTime.Local().Format(time.RFC3339), "", ""})
				total++
			}
		}

		if !isTruncated || total >= limit {
			table.SetFooter([]string{"", "", "", "", "", "", "Total Objects: ", fmt.Sprintf("%d", total)})
			table.Render()
			break
		}
		table.Render()

		// 重置表格
		table = tablewriter.NewWriter(os.Stdout)
		table.SetBorder(false)
		table.SetAlignment(tablewriter.ALIGN_LEFT)
		table.SetAutoWrapText(false)
	}

	return nil
}

func ListOfsObjects(c *cos.Client, cosUrl StorageUrl, limit int, recursive bool, filters []FilterOptionType) error {
	lsCounter := &LsCounter{}
	prefix := cosUrl.(*CosUrl).Object

	lsCounter.Table = tablewriter.NewWriter(os.Stdout)
	lsCounter.Table.SetHeader([]string{"Key", "Type", "Last Modified", "Etag", "Size", "RestoreStatus"})
	lsCounter.Table.SetBorder(false)
	lsCounter.Table.SetAlignment(tablewriter.ALIGN_LEFT)
	lsCounter.Table.SetAutoWrapText(false)

	err := getOfsObjects(c, prefix, limit, recursive, filters, "", lsCounter)
	if err != nil {
		return err
	}

	lsCounter.Table.SetFooter([]string{"", "", "", "", "Total Objects: ", fmt.Sprintf("%d", lsCounter.TotalLimit)})
	lsCounter.Table.Render()
	return nil
}

func getOfsObjects(c *cos.Client, prefix string, limit int, recursive bool, filters []FilterOptionType, marker string, lsCounter *LsCounter) error {
	var err error
	var objects []cos.Object
	var commonPrefixes []string
	isTruncated := true

	for isTruncated {

		queryLimit := 1000
		if limit-lsCounter.TotalLimit < 1000 {
			queryLimit = limit - lsCounter.TotalLimit
		}

		if queryLimit <= 0 {
			return nil
		}

		err, objects, commonPrefixes, isTruncated, marker = getOfsObjectListForLs(c, prefix, marker, queryLimit, recursive)

		if err != nil {
			return fmt.Errorf("list objects error : %v", err)
		}

		for _, object := range objects {
			object.Key, _ = url.QueryUnescape(object.Key)
			if cosObjectMatchPatterns(object.Key, filters) {
				utcTime, err := time.Parse(time.RFC3339, object.LastModified)
				if err != nil {
					return fmt.Errorf("Error parsing time:%v", err)
				}
				if lsCounter.TotalLimit >= limit {
					break
				}
				lsCounter.TotalLimit++
				lsCounter.RenderNum++
				lsCounter.Table.Append([]string{object.Key, object.StorageClass, utcTime.Local().Format(time.RFC3339), object.ETag, formatBytes(float64(object.Size)), object.RestoreStatus})
				tableRender(lsCounter)
			}
		}

		if len(commonPrefixes) > 0 {
			for _, commonPrefix := range commonPrefixes {
				commonPrefix, _ = url.QueryUnescape(commonPrefix)
				if lsCounter.TotalLimit >= limit {
					break
				}
				if cosObjectMatchPatterns(commonPrefix, filters) {
					lsCounter.TotalLimit++
					lsCounter.RenderNum++
					lsCounter.Table.Append([]string{commonPrefix, "DIR", "", "", "", ""})
					tableRender(lsCounter)
				}
				if recursive {
					// 递归目录
					err = getOfsObjects(c, commonPrefix, limit, recursive, filters, "", lsCounter)
					if err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}

func tableRender(lsCounter *LsCounter) {
	if lsCounter.RenderNum >= OfsMaxRenderNum {
		lsCounter.Table.Render()
		lsCounter.Table.ClearRows()
		lsCounter.RenderNum = 0
		lsCounter.Table = tablewriter.NewWriter(os.Stdout)
		lsCounter.Table.SetBorder(false)
		lsCounter.Table.SetAlignment(tablewriter.ALIGN_LEFT)
		lsCounter.Table.SetAutoWrapText(false)
	}
}

func ListBuckets(c *cos.Client, limit int) error {
	var buckets []cos.Bucket
	marker := ""
	isTruncated := true
	totalNum := 0
	var err error

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Bucket Name", "Region", "Create Date"})
	for isTruncated {
		buckets, marker, isTruncated, err = GetBucketsList(c, limit, marker)
		if err != nil {
			return err
		}
		for _, b := range buckets {
			table.Append([]string{b.Name, b.Region, b.CreationDate})
			totalNum++
		}
		if limit > 0 {
			isTruncated = false
		}
	}

	table.SetFooter([]string{"", "Total Buckets: ", fmt.Sprintf("%d", totalNum)})
	table.SetBorder(false)
	table.Render()

	return err
}

func GetBucketsList(c *cos.Client, limit int, marker string) (buckets []cos.Bucket, nextMarker string, isTruncated bool, err error) {
	opt := &cos.ServiceGetOptions{
		Marker:  marker,
		MaxKeys: int64(limit),
	}
	res, _, err := c.Service.Get(context.Background(), opt)

	if err != nil {
		return buckets, nextMarker, isTruncated, err
	}

	buckets = res.Buckets
	nextMarker = res.NextMarker
	isTruncated = res.IsTruncated

	return
}
