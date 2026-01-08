package officialaccount

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/gorilla/websocket"
)

type Client struct {
	hub  *OfficialAccountClient
	conn *websocket.Conn
	send chan []byte
}

func (c *Client) write_pump() {
	ticker := time.NewTicker(5 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func ValidateTokenFilepath(file_path string, root_dir string) (string, error) {
	token_filepath := file_path
	if file_path == "" {
		return "", nil
	}
	if !filepath.IsAbs(file_path) {
		token_filepath = filepath.Join(root_dir, file_path)
	}
	if _, err := os.Stat(token_filepath); err != nil {
		return "", err
	}
	// script_byte, err := os.ReadFile(token_filepath)
	// if err != nil {
	// 	return err
	// }
	// settings.InjectExtraScriptAfterJSMain = string(script_byte)
	return token_filepath, nil
}

type APIClientWSMessage struct {
	Type  string      `json:"type"`
	Data  interface{} `json:"data"`
	Error string      `json:"error"`
}

type ClientWSMessage struct {
	Type string `json:"type"`
	Data string `json:"data"`
}
type ClientWebsocketRequestBody struct {
	ID   string      `json:"id"`
	Key  string      `json:"key"`
	Body interface{} `json:"data"`
}
type ClientWebsocketResponse struct {
	Id string `json:"id"`
	// 调用 wx api 原始响应
	Data json.RawMessage `json:"data"`
}

type OfficialMsgListResp struct {
	Ret        int    `json:"ret"`
	ErrMsg     string `json:"errmsg"`
	MsgList    string `json:"general_msg_list"`
	HasMore    int    `json:"can_msg_continue"`
	MsgCount   int    `json:"msg_count"`
	NextOffset int    `json:"next_offset"`
}
type CommonMsgInfo struct {
	Id       int    `json:"id"`
	Type     int    `json:"type"`
	Datetime int    `json:"datetime"`
	Fakeid   string `json:"fakeid"`
	Status   int    `json:"status"`
	Content  string `json:"content"`
}
type OfficialAccountMsgListRespItem struct {
	MsgExtInfo    OfficialAccountMsg `json:"app_msg_ext_info"`
	CommonMsgInfo CommonMsgInfo      `json:"comm_msg_info"`
}
type OfficialArticle struct {
	Title                  string `json:"title"`
	Digest                 string `json:"digest"`
	Content                string `json:"content"`
	Fileid                 int    `json:"fileid"`
	ContentUrl             string `json:"content_url"`
	SourceUrl              string `json:"source_url"`
	Cover                  string `json:"cover"`
	Author                 string `json:"author"`
	CopyrightStat          int    `json:"copyright_stat"`
	DelFlag                int    `json:"del_flag"`
	ItemShowType           int    `json:"item_show_type"`
	AudioFileid            int    `json:"audio_fileid"`
	Duration               int    `json:"duration"`
	PlayUrl                string `json:"play_url"`
	MaliciousTitleReasonId int    `json:"malicious_title_reason_id"`
	MaliciousContentType   int    `json:"malicious_content_type"`
}

// 推送，就是比 OfficialArticle 多了 IsMulti 和 MultiAppMsgItemList 两个字段
type OfficialAccountMsg struct {
	Title                  string            `json:"title"`
	Digest                 string            `json:"digest"`
	Content                string            `json:"content"`
	Fileid                 int               `json:"fileid"`
	ContentUrl             string            `json:"content_url"`
	SourceUrl              string            `json:"source_url"`
	Cover                  string            `json:"cover"`
	Subtype                int               `json:"subtype"`
	IsMulti                int               `json:"is_multi"`
	MultiAppMsgItemList    []OfficialArticle `json:"multi_app_msg_item_list"`
	Author                 string            `json:"author"`
	CopyrightStat          int               `json:"copyright_stat"`
	Duration               int               `json:"duration"`
	DelFlag                int               `json:"del_flag"`
	ItemShowType           int               `json:"item_show_type"`
	AudioFileid            int               `json:"audio_fileid"`
	PlayUrl                string            `json:"play_url"`
	MaliciousTitleReasonId int               `json:"malicious_title_reason_id"`
	MaliciousContentType   int               `json:"malicious_content_type"`
}
