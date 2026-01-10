package pages

import (
	"fmt"
)

type DeployBody struct {
	Directory   string `json:"directory"`
	JWT         string `json:"jwt"`
	AccountId   string `json:"account_id"`
	ProjectName string `json:"project_name"`
}

// https://github.com/cloudflare/workers-sdk/blob/main/packages/wrangler/src/api/pages/deploy.ts#L102
func Deploy(body DeployBody) error {
	file_map, err := Validate(body.Directory)
	if err != nil {
		return err
	}
	manifest, err := Upload(UploadPayload{
		FilesMap: file_map,
		JWT:      body.JWT,
	})
	if err != nil {
		return err
	}
	fmt.Println("before Api_create_deployment", manifest.Files)
	Api_create_deployment(DeploymentBody{
		AccountId:   body.AccountId,
		ProjectName: body.ProjectName,
		Manifest:    manifest.Files,
		// Buffer:      &buf,
	})
	var files_hash []string
	for _, file := range file_map {
		files_hash = append(files_hash, file.Hash)
	}
	_, err = Api_upsert_hashes(files_hash, body.JWT)
	if err != nil {
		return err
	}
	return nil
}
