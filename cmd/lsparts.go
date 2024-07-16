package cmd

import (
	"coscli/util"
	"fmt"
	"os"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var lspartsCmd = &cobra.Command{
	Use:   "lsparts",
	Short: "List multipart uploads",
	Long: `List multipart uploads

Format:
  ./coscli lsparts cos://<bucket-name>[/<prefix>] [flags]

Example:
  ./coscli lsparts cos://examplebucket/test/`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		limit, _ := cmd.Flags().GetInt("limit")
		include, _ := cmd.Flags().GetString("include")
		exclude, _ := cmd.Flags().GetString("exclude")
		if limit < 0 || limit > 1000 {
			return fmt.Errorf("Flag --limit should in range 0~1000")
		}

		err := listParts(args[0], limit, include, exclude)
		return err
	},
}

func init() {
	rootCmd.AddCommand(lspartsCmd)

	lspartsCmd.Flags().Int("limit", 0, "Limit the number of parts listed(0~1000)")
	lspartsCmd.Flags().String("include", "", "List files that meet the specified criteria")
	lspartsCmd.Flags().String("exclude", "", "Exclude files that meet the specified criteria")
}

func listParts(arg string, limit int, include string, exclude string) error {
	bucketName, cosPath := util.ParsePath(arg)
	c, err := util.NewClient(&config, &param, bucketName)
	if err != nil {
		return err
	}

	uploads, err := util.GetUploadsListRecursive(c, cosPath, limit, include, exclude)
	if err != nil {
		return err
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Key", "Upload ID", "Initiate time"})
	for _, u := range uploads {
		table.Append([]string{u.Key, u.UploadID, u.Initiated})
	}
	table.SetBorder(false)
	table.SetAlignment(tablewriter.ALIGN_RIGHT)
	table.SetFooter([]string{"", "", fmt.Sprintf("Total: %d", len(uploads))})
	table.Render()
	return nil
}
