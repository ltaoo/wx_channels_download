package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"wx_channel/pkg/decrypt"
	"wx_channel/pkg/download"
)

var (
	video_url         string
	filename          string
	video_decrypt_key int
)

var download_cmd = &cobra.Command{
	Use:   "download",
	Short: "下载视频",
	Long:  "从指定URL下载视频文件",
	Run: func(cmd *cobra.Command, args []string) {
		command := cmd.Name()
		if command != "download" {
			return
		}
		download_command(DownloadCommandArgs{
			URL:        video_url,
			DecryptKey: video_decrypt_key,
			Filename:   filename,
		})
	},
}

func init() {
	now := int(time.Now().Unix())
	download_cmd.Flags().StringVar(&video_url, "url", "", "视频URL（必需）")
	download_cmd.Flags().IntVar(&video_decrypt_key, "key", 0, "解密密钥（未加密的视频不用传该参数）")
	download_cmd.Flags().StringVar(&filename, "filename", strconv.Itoa(now)+".mp4", "下载后的文件名")
	download_cmd.MarkFlagRequired("url")

	root_cmd.AddCommand(download_cmd)
}

type DownloadCommandArgs struct {
	URL        string
	Filename   string
	DecryptKey int
}

func download_command(args DownloadCommandArgs) {
	url := args.URL
	homedir, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("[ERROR]获取下载路径失败 %v\n", err.Error())
		return
	}
	tmp_filename := "tmp_wx_" + strconv.Itoa(int(time.Now().Unix()))
	tmp_dest_filepath := filepath.Join(homedir, "Downloads", tmp_filename)
	dest_filepath := filepath.Join(homedir, "Downloads", args.Filename)

	if args.DecryptKey == 0 {
		tmp_dest_filepath = dest_filepath
	}

	if err := download.MultiThreadingDownload(url, 4, tmp_dest_filepath, tmp_dest_filepath); err != nil {
		fmt.Printf("[ERROR]%v\n", err.Error())
		return
	}
	// elapsed := time.Since(start_time)
	// speed := float64(file_size) / elapsed.Seconds() / 1024 / 1024

	// fmt.Printf("文件大小: %.2f MB\n", float64(file_size)/1024/1024)
	// fmt.Printf("耗时: %.2f 秒\n", elapsed.Seconds())
	// fmt.Printf("平均速度: %.2f MB/s\n", speed)

	if args.DecryptKey != 0 {
		fmt.Printf("下载完成!\n")
		fmt.Printf("开始对临时文件解密 %s\n", tmp_dest_filepath)
		length := uint32(131072)
		key := uint64(args.DecryptKey)
		data, err := os.ReadFile(tmp_dest_filepath)
		if err != nil {
			fmt.Printf("[ERROR]读取临时文件失败 %v\n", err.Error())
			return
		}
		decrypt.DecryptData(data, length, key)
		err = os.WriteFile(dest_filepath, data, 0644)
		if err != nil {
			fmt.Printf("[ERROR]写入文件失败 %v\n", err.Error())
			return
		}
		fmt.Printf("删除临时文件 %s\n", tmp_dest_filepath)
		if err := os.Remove(tmp_dest_filepath); err != nil {
			if os.IsNotExist(err) {
				fmt.Println("[ERROR]临时文件不存在")
			} else if os.IsPermission(err) {
				fmt.Println("[ERROR]没有权限删除临时文件")
			} else {
				fmt.Printf("[ERROR]临时文件删除失败 %v\n", err.Error())
			}
		}
		fmt.Printf("解密完成，文件路径为 %s\n", dest_filepath)
		return
	}
	fmt.Printf("下载完成，文件路径为 %s\n", dest_filepath)
}
