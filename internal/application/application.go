// deprecated
package application

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/fatih/color"

	"wx_channel/pkg/certificate"
	"wx_channel/pkg/decrypt"
	"wx_channel/pkg/proxy"
)

type Biz struct {
	Version string
	Debug   bool
}

func NewBiz(version string) *Biz {
	return &Biz{
		Version: version,
		// Files:   files,
	}
}
func (a *Biz) SetDebug(debug bool) {
	a.Debug = debug
}

func (a *Biz) UninstallCertificate(cert_file_name string) {
	settings := proxy.ProxySettings{}
	if err := proxy.DisableProxy(settings); err != nil {
		fmt.Printf("\nERROR 取消代理失败 %v\n", err.Error())
		return
	}
	if err := certificate.UninstallCertificate(cert_file_name); err != nil {
		fmt.Printf("\nERROR 删除根证书失败 %v\n", err.Error())
		return
	}
	color.Green(fmt.Sprintf("\n\n删除根证书 '%v' 成功\n", cert_file_name))
}

type DecryptCOmmandArgs struct {
	FilePath   string
	DecryptKey int
}

func (a *Biz) DecryptCannelFile(args DecryptCOmmandArgs) error {
	length := uint32(131072)
	key := uint64(args.DecryptKey)
	data, err := os.ReadFile(args.FilePath)
	if err != nil {
		return fmt.Errorf("[ERROR]读取已下载的文件失败 %v\n", err.Error())
	}
	decrypt.DecryptData(data, length, key)
	err = os.WriteFile(args.FilePath, data, 0644)
	if err != nil {
		return fmt.Errorf("[ERROR]写入文件失败 %v\n", err.Error())
	}
	return nil
}

type DownloadCommandArgs struct {
	URL        string
	Filename   string
	DecryptKey int
}

func (a *Biz) DownloadChannelFile(args DownloadCommandArgs) {
	resp, err := http.Get(args.URL)
	if err != nil {
		fmt.Printf("[ERROR]下载失败 %v\n", err.Error())
		return
	}
	defer resp.Body.Close()
	homedir, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("[ERROR]获取下载路径失败 %v\n", err.Error())
		return
	}
	tmp_filename := "wx_" + strconv.Itoa(int(time.Now().Unix()))
	tmp_dest_filepath := path.Join(homedir, "Downloads", tmp_filename)
	dest_filepath := path.Join(homedir, "Downloads", args.Filename)
	file, err := os.Create(tmp_dest_filepath)
	if err != nil {
		fmt.Printf("[ERROR]下载文件失败 %v\n", err.Error())
		os.Exit(0)
		return
	}
	defer file.Close()
	content_length := resp.Header.Get("Content-Length")
	total_size := int64(-1)
	if content_length != "" {
		total_size, _ = strconv.ParseInt(content_length, 10, 64)
	}
	buf := make([]byte, 32*1024) // 32KB buffer
	var downloaded int64 = 0
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			_, werr := file.Write(buf[:n])
			if werr != nil {
				fmt.Printf("[ERROR]写入文件失败 %v\n", werr.Error())
				return
			}
			downloaded += int64(n)
			if total_size > 0 {
				percent := float64(downloaded) / float64(total_size) * 100
				fmt.Printf("\r\033[K已下载: %d/%d 字节 (%.2f%%)", downloaded, total_size, percent)
			} else {
				fmt.Printf("\r\033[K已下载: %d 字节", downloaded)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Printf("[ERROR]下载文件失败2 %v\n", err.Error())
			return
		}
	}
	fmt.Println()
	if args.DecryptKey != 0 {
		fmt.Printf("开始对文件解密 %s", tmp_dest_filepath)
		length := uint32(131072)
		enclen_str := resp.Header.Get("X-enclen")
		if enclen_str != "" {
			v, err := strconv.ParseUint(enclen_str, 10, 32)
			if err == nil {
				length = uint32(v)
			}
		}
		key := uint64(args.DecryptKey)
		data, err := os.ReadFile(tmp_dest_filepath)
		if err != nil {
			fmt.Printf("[ERROR]读取已下载的文件失败 %v\n", err.Error())
			return
		}
		decrypt.DecryptData(data, length, key)
		err = os.WriteFile(dest_filepath, data, 0644)
		if err != nil {
			fmt.Printf("[ERROR]写入文件失败 %v\n", err.Error())
			return
		}
		file.Close()
		err = os.Remove(tmp_dest_filepath)
		if err != nil {
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
	file.Close()
	err = os.Rename(tmp_dest_filepath, dest_filepath)
	if err != nil {
		fmt.Printf("[ERROR]重命名文件失败 %v\n", err.Error())
		return
	}
	fmt.Printf("下载完成，件路径为 %s\n", dest_filepath)
}
