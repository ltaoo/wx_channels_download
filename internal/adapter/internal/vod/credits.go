package vod

import (
	"encoding/json"
	"strings"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	"wx_channel/pkg/scraper"
)

func content_influencers(credits []scraper.Credit, platform string) []adapter.ContentInfluencerReference {
	var references []adapter.ContentInfluencerReference
	indexes := make(map[string]int)
	seen := make(map[string]bool)
	for _, credit := range credits {
		name, role := strings.TrimSpace(credit.Name), strings.TrimSpace(credit.Role)
		key := name + "\x00" + role + "\x00" + credit.Character
		if name == "" || role == "" || seen[key] {
			continue
		}
		seen[key] = true
		index, exists := indexes[name]
		if !exists {
			metadata, _ := json.Marshal(map[string]string{"source": platform, "source_person_id": credit.ID})
			index = len(references)
			indexes[name] = index
			references = append(references, adapter.ContentInfluencerReference{Influencer: &model.Influencer{Name: name, AvatarURL: credit.AvatarURL, KnownForDepartment: role, MetadataJSON: string(metadata)}})
		}
		metadata, _ := json.Marshal(map[string]string{"source": platform, "source_person_id": credit.ID, "character": credit.Character})
		references[index].Roles = append(references[index].Roles, adapter.ContentInfluencerRole{Role: role, SortOrder: credit.SortOrder, MetadataJSON: string(metadata)})
	}
	return references
}
