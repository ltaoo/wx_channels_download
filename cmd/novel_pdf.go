package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	contentshuba69 "wx_channel/pkg/contentplatform/69shuba"
)

var (
	novelPDFDir    string
	novelPDFOutput string
	novelPDFForce  bool
)

var novelPDFCmd = &cobra.Command{
	Use:   "novel-pdf",
	Short: "从本地 69shuba 小说目录生成 PDF",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := strings.TrimSpace(novelPDFDir)
		if dir == "" && len(args) > 0 {
			dir = args[0]
		}
		if dir == "" {
			return fmt.Errorf("缺少本地小说目录")
		}
		output := strings.TrimSpace(novelPDFOutput)
		if output == "" {
			output = filepath.Join(dir, filepath.Base(dir)+".pdf")
		}
		if err := contentshuba69.GenerateLocalArchivePDFWithOptions(context.Background(), dir, output, contentshuba69.PDFOptions{ForceClean: novelPDFForce}); err != nil {
			return err
		}
		fmt.Printf("PDF 已生成: %s\n", output)
		return nil
	},
}

func init() {
	novelPDFCmd.Flags().StringVar(&novelPDFDir, "dir", "", "本地 69shuba 小说目录")
	novelPDFCmd.Flags().StringVarP(&novelPDFOutput, "output", "o", "", "输出 PDF 路径")
	novelPDFCmd.Flags().BoolVar(&novelPDFForce, "force-clean", false, "强制重建 clean/catalog.json 和章节 txt")
	root_cmd.AddCommand(novelPDFCmd)
}
