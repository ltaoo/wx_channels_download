package nodes

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"wx_channel/pkg/flowengine/engine"
)

type DownloadNode struct {
	Id     string
	Config map[string]interface{}
}

// download --url "https://finder.video.qq.com/251/20302/stodownload?encfilekey=Cvvj5Ix3eez3Y79SxtvVL0L7CkPM6dFibFeI6caGYwFHic7EtDI76DS6KL6rtRm0r98K0tXFINfh1V3M7TLGiaIterJ9CVXH0KyhBFJf8JrdOibTrFlTcTBAuSRhRibE2CCf8nzO6Iar7sUskG1oLrlyFSg&hy=SH&idx=1&m=836cfd7ec7a6f0107e31ec94005c94b6&uzid=7a166&token=AxricY7RBHdXcdmXnu6AbhCckEUzvhPJo18S2UZnkjFlzBHNctAGwpEv4MrSaPfb7rrViakebGPZao4D6gRWER9J4BCBNfciaL8M7RAL0ZgJIBxLJl8Tw2WwME2tbLlQGNQxdDOjqKjfxGRkdVK8ibNqGErtibhWvNZ0Iv7o94icFnbbg&basedata=CAESABoDeFYwIgAqBwiLKRAAGAI&sign=acxsHufIiuQel7YV1ZNa7yubDkqwKVfLZHw4oFsnb6dCG2wLyFVt_WxxwY0w0nlsjSOJ3BaI-XdlQ4KO7pqceg&ctsc=146&web=1&extg=10f0000&svrbypass=AAuL%2FQsFAAABAAAAAACZGBObJyZPtdNen8Y3aRAAAADnaHZTnGbFfAj9RgZXfw6VptrV3n5cbF33AQwEFraPOyfy%2BN3LhWEBiFnUAN2iP7KSZ%2FLq1Czu5SM%3D&svrnonce=1765263007" --key 2145623499 --filename "【英雄联盟豹女cos】豹女也是喵 #英雄联盟 #cosplay_original.mp4"
func NewDownloadNode(config map[string]interface{}) engine.Node {
	id, _ := config["id"].(string)
	return &DownloadNode{Id: id, Config: config}
}

func (n *DownloadNode) ID() string   { return n.Id }
func (n *DownloadNode) Type() string { return "DownloadNode" }

func (n *DownloadNode) Execute(ctx *engine.ProcessContext) (bool, []string, error) {
	urlVal, _ := ctx.Data["url"].(string)
	if urlVal == "" {
		return false, nil, errors.New("missing download_url")
	}
	filenameVal, _ := ctx.Data["filename"].(string)
	if filenameVal == "" {
		return false, nil, errors.New("missing filename")
	}
	outDir := "downloads"
	if v, ok := n.Config["output_dir"].(string); ok && v != "" {
		outDir = v
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return false, nil, err
	}
	// filename := deriveFilename(urlVal)
	fullpath := filepath.Join(outDir, filenameVal)
	if err := httpDownload(urlVal, fullpath); err != nil {
		return false, nil, err
	}
	ctx.Data["local_path"] = fullpath
	next := ctx.EngineRef.GetNextNodeIDsFromDefinition(ctx, n.Id)
	fmt.Println("next nodes", next)
	return true, next, nil
}
