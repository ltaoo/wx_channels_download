package cmd

import (
	"fmt"
	"os"
	"wx_channel/pkg/decrypt"

	"github.com/spf13/cobra"
)

var (
	file_path          string
	video_decrypt_key2 int
)
var decrypt_cmd = &cobra.Command{
	Use:   "decrypt",
	Short: "解密视频",
	Long:  "使用 key 对本地加密视频进行解密",
	Run: func(cmd *cobra.Command, args []string) {
		command := cmd.Name()
		if command != "decrypt" {
			return
		}
		decrypt_command(DecryptCOmmandArgs{
			Filepath:   file_path,
			DecryptKey: video_decrypt_key2,
		})
	},
}

func init() {
	decrypt_cmd.Flags().StringVar(&file_path, "filepath", "", "视频文件地址（必需）")
	decrypt_cmd.Flags().IntVar(&video_decrypt_key2, "key", 0, "解密密钥（必需）")
	decrypt_cmd.MarkFlagRequired("filepath")

	root_cmd.AddCommand(decrypt_cmd)
}

type DecryptCOmmandArgs struct {
	Filepath   string
	DecryptKey int
}

func decrypt_command(args DecryptCOmmandArgs) {
	if args.Filepath == "" {
		fmt.Printf("[ERROR]文件路径不能为空\n")
		return
	}
	if args.DecryptKey == 0 {
		fmt.Printf("[ERROR]解密密钥不能为空\n")
		return
	}
	fmt.Printf("开始对文件解密 %s\n", args.Filepath)
	length := uint32(131072)
	key := uint64(args.DecryptKey)
	data, err := os.ReadFile(args.Filepath)
	if err != nil {
		fmt.Printf("[ERROR]读取已下载的文件失败 %v\n", err.Error())
		return
	}
	decrypt.DecryptData(data, length, key)
	err = os.WriteFile(args.Filepath, data, 0644)
	if err != nil {
		fmt.Printf("[ERROR]写入文件失败 %v\n", err.Error())
		return
	}
	fmt.Printf("解密完成 %s\n", args.Filepath)
}
