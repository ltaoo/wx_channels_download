package download

import (
	"context"
	"errors"
)

var (
	ErrUnsupportedURL     = errors.New("unsupported url")
	ErrResolveUnavailable = errors.New("resolve unavailable")
	ErrVariantNotFound    = errors.New("variant not found")
)

const InternalKeyDisableJSONVariant = "disable_json_variant"

type Handler interface {
	Platform() string
	Match(rawURL string) bool
	Probe(ctx context.Context, input ProbeInput) (*Probe, error)
	Resolve(ctx context.Context, input ResolveInput) (*ResolvedRequest, error)
	Plan(ctx context.Context, resolved *ResolvedRequest) (*PipelinePlan, error)
}

// ProbeInput is the URL and optional platform-specific context used by a
// handler to fetch metadata before user confirmation.
type ProbeInput struct {
	// URL is the original user-provided content URL.
	URL string `json:"url"`
	// Extra carries adapter-specific request data that is not part of the
	// shared download contract.
	Extra map[string]any `json:"extra,omitempty"`
}

// ResolveInput contains the selected probe/options needed to produce a
// concrete download request.
type ResolveInput struct {
	// URL is the original content URL. It is used when Probe is absent or when
	// a handler needs to preserve the source request.
	URL string `json:"url"`
	// Probe is the previously fetched pre-confirmation result. Handlers should
	// reuse it to avoid fetching the same platform data twice.
	Probe *Probe `json:"probe,omitempty"`
	// Options contains user-selected variant and output preferences.
	Options Options `json:"options,omitempty"`
	// Extra carries adapter-specific data passed through from the caller.
	Extra map[string]any `json:"extra,omitempty"`
}

// Options describes user choices from the confirmation step.
type Options struct {
	// VariantID selects one item from Probe.Variants.
	VariantID string `json:"variant_id,omitempty"`
	// Spec is a platform-specific rendition or quality key, such as a video
	// stream format parameter.
	Spec string `json:"spec,omitempty"`
	// Suffix overrides the output file suffix, including the leading dot.
	Suffix string `json:"suffix,omitempty"`
	// Filename overrides the output base filename.
	Filename string `json:"filename,omitempty"`
	// SaveDir overrides the default download directory for this request.
	SaveDir string `json:"save_dir,omitempty"`
	// Policy groups higher-level quality/format preferences.
	Policy Policy `json:"policy,omitempty"`
	// Extra carries platform-specific options that are not part of the shared
	// selection model.
	Extra map[string]any `json:"extra,omitempty"`
}

// Policy captures generic output preferences that a handler may translate into
// a concrete variant or pipeline.
type Policy struct {
	// Quality is the preferred quality level, such as best, original, or a
	// platform-specific quality name.
	Quality string `json:"quality,omitempty"`
	// Format is the preferred media/container format, such as mp4, mp3, or html.
	Format string `json:"format,omitempty"`
}

// Probe is the pre-confirmation metadata returned by a platform handler.
type Probe struct {
	// ID is an optional workflow/probe identifier assigned by the caller.
	ID string `json:"id,omitempty"`
	// Platform is the stable content platform identifier, such as zhihu or wx_channels.
	Platform string `json:"platform"`
	// SourceURL is the URL submitted by the user.
	SourceURL string `json:"source_url"`
	// CanonicalURL is the normalized platform URL when the handler can derive one.
	CanonicalURL string `json:"canonical_url,omitempty"`
	// ContentID is the platform's stable primary identifier for this content.
	ContentID string `json:"content_id,omitempty"`
	// Content is the platform-fetched content payload passed through to the API
	// and later Resolve step. Handlers usually store a *Content[T] here.
	Content any `json:"content,omitempty"`
	// Variants lists the downloadable choices shown to the user.
	Variants []Variant `json:"variants,omitempty"`
	// Defaults stores the handler-recommended default selection.
	Defaults Defaults `json:"defaults,omitempty"`
	// Internal stores probe-time execution data reused by Resolve and executors.
	// It may contain large parsed objects and is never serialized.
	Internal map[string]any `json:"-"`
	// Warnings contains non-fatal probe messages that the UI may display.
	Warnings []string `json:"warnings,omitempty"`
}

// Defaults is the handler-provided default option set for a probe.
type Defaults struct {
	// VariantID is the default Variant.ID to select.
	VariantID string `json:"variant_id,omitempty"`
	// Spec is the default platform-specific rendition key.
	Spec string `json:"spec,omitempty"`
	// Suffix is the default output file suffix, including the leading dot.
	Suffix string `json:"suffix,omitempty"`
}

// Variant describes one downloadable output choice for a probed content item.
type Variant struct {
	// ID is the stable selection key submitted back by the user.
	ID string `json:"id"`
	// Type is the output category, such as video, audio, image, archive, or html.
	Type string `json:"type"`
	// Label is the user-facing option name.
	Label string `json:"label"`
	// Spec is a platform-specific rendition key used during Resolve.
	Spec string `json:"spec,omitempty"`
	// Suffix is the expected output file suffix, including the leading dot.
	Suffix string `json:"suffix,omitempty"`
	// Size is the expected output size in bytes when known.
	Size int64 `json:"size,omitempty"`
	// Width is the media width in pixels when known.
	Width int `json:"width,omitempty"`
	// Height is the media height in pixels when known.
	Height int `json:"height,omitempty"`
	// Bitrate is the media bitrate in bits per second when known.
	Bitrate int `json:"bitrate,omitempty"`
	// Requires lists external tools or capabilities needed for this variant.
	Requires []string `json:"requires,omitempty"`
	// Metadata stores variant-specific details for handlers or UI consumers.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// ResolvedRequest is the concrete download request produced after user
// confirmation.
type ResolvedRequest struct {
	// Platform is the stable content platform identifier.
	Platform string `json:"platform"`
	// SourceURL is the original URL used to create the request.
	SourceURL string `json:"source_url"`
	// CanonicalURL is the normalized platform URL when available.
	CanonicalURL string `json:"canonical_url,omitempty"`
	// ContentID is the platform's stable primary identifier for this content.
	ContentID string `json:"content_id,omitempty"`
	// Title is the final user-facing title associated with the task.
	Title string `json:"title,omitempty"`
	// Filename is the requested output base filename.
	Filename string `json:"filename,omitempty"`
	// Suffix is the final output suffix, including the leading dot.
	Suffix string `json:"suffix,omitempty"`
	// Download is the concrete source request to execute.
	Download DownloadSpec `json:"download"`
	// Labels stores string metadata persisted with the task and used by legacy
	// download/content records.
	Labels map[string]string `json:"labels,omitempty"`
	// Metadata stores internal execution data, such as selected variant IDs or
	// parsed platform objects reused by custom executors.
	Metadata map[string]any `json:"metadata,omitempty"`
	// Content is the platform-fetched content payload associated with the task.
	Content any `json:"content,omitempty"`
	// Pipeline is the platform-declared processing graph for this request.
	Pipeline *PipelinePlan `json:"pipeline,omitempty"`
}

// DownloadSpec describes the exact source fetch that a SourceExecutor should
// perform.
type DownloadSpec struct {
	// URL is the concrete source URL, including custom protocol URLs such as
	// zhihu:// or zip://.
	URL string `json:"url"`
	// Method is the HTTP-style request method. Empty means the executor default.
	Method string `json:"method,omitempty"`
	// Headers contains request headers required by the source.
	Headers map[string]string `json:"headers,omitempty"`
	// Body contains the optional request payload for non-GET style sources.
	Body []byte `json:"body,omitempty"`
	// Protocol selects the SourceExecutor, such as http, zip, zhihu, or
	// officialaccount.
	Protocol string `json:"protocol,omitempty"`
	// Connections is the requested parallel connection count when supported.
	Connections int `json:"connections,omitempty"`
}

// Content is a typed platform content envelope. T is owned by the platform
// package and can be a raw response object, a structured page model, or a
// composition of multiple content objects.
type Content[T any] struct {
	// Summary is a small cross-platform projection used for filenames, lists,
	// and confirmation UI.
	Summary ContentSummary `json:"summary,omitempty"`
	// Data is the platform-specific content payload.
	Data T `json:"data,omitempty"`
	// Metadata stores content-owned auxiliary IDs and attributes, such as
	// question_id, nonce_id, or publish_time.
	Metadata map[string]any `json:"metadata,omitempty"`
	// Output stores optional probe output derived from the content, such as
	// body_html for text platforms.
	Output map[string]any `json:"output,omitempty"`
}

// ContentSummary is the small content projection shared across platform
// content types.
type ContentSummary struct {
	// Platform is the content platform identifier.
	Platform string `json:"platform,omitempty"`
	// Type classifies the content, such as video, image_album, answer,
	// question, or article.
	Type string `json:"type,omitempty"`
	// ID is the platform's stable primary identifier for this content.
	ID string `json:"id,omitempty"`
	// Title is the content title.
	Title string `json:"title,omitempty"`
	// Description is the content summary, excerpt, or author-provided
	// description.
	Description string `json:"description,omitempty"`
	// Author is a short display name for the content author.
	Author string `json:"author,omitempty"`
	// URL is the canonical or playable platform URL for the content.
	URL string `json:"url,omitempty"`
	// SourceURL is the original or canonical source page URL.
	SourceURL string `json:"source_url,omitempty"`
	// AuthorNickname is the display name of the content author.
	AuthorNickname string `json:"author_nickname,omitempty"`
	// AuthorAvatarURL is the author's avatar image URL.
	AuthorAvatarURL string `json:"author_avatar_url,omitempty"`
	// CoverURL is the preview image or representative thumbnail URL.
	CoverURL string `json:"cover_url,omitempty"`
	// Duration is the content duration in seconds when the platform provides it.
	Duration int64 `json:"duration,omitempty"`
}

// PipelinePlan declares the platform-specific processing graph to run after
// Resolve.
type PipelinePlan struct {
	// Platform is the content platform that owns this plan.
	Platform string `json:"platform"`
	// Nodes is the ordered list of pipeline steps and dependencies.
	Nodes []PipelineNode `json:"nodes"`
	// Metadata stores plan-level execution hints shared by pipeline nodes.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// PipelineNode describes one executable step in a PipelinePlan.
type PipelineNode struct {
	// ID is the unique node identifier within the plan.
	ID string `json:"id"`
	// Type selects the executor behavior, such as download_asset,
	// sanitize_html, or persist_artifacts.
	Type string `json:"type"`
	// Stage groups the node by lifecycle phase, such as download, post, or
	// persist.
	Stage string `json:"stage,omitempty"`
	// DependsOn lists node IDs that must complete before this node runs.
	DependsOn []string `json:"depends_on,omitempty"`
	// Args contains node-specific executor arguments.
	Args map[string]any `json:"args,omitempty"`
}

func SelectVariant(probe *Probe, options Options) (*Variant, error) {
	if probe == nil || len(probe.Variants) == 0 {
		return nil, ErrVariantNotFound
	}
	variantID := options.VariantID
	if variantID == "" {
		variantID = probe.Defaults.VariantID
	}
	if variantID != "" {
		for i := range probe.Variants {
			if probe.Variants[i].ID == variantID {
				return &probe.Variants[i], nil
			}
		}
	}
	if options.Spec != "" || options.Suffix != "" {
		for i := range probe.Variants {
			sameSpec := options.Spec == "" || probe.Variants[i].Spec == options.Spec
			sameSuffix := options.Suffix == "" || probe.Variants[i].Suffix == options.Suffix
			if sameSpec && sameSuffix {
				return &probe.Variants[i], nil
			}
		}
	}
	return &probe.Variants[0], nil
}

func MergeLabels(base map[string]string, pairs map[string]string) map[string]string {
	out := make(map[string]string, len(base)+len(pairs))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range pairs {
		out[k] = v
	}
	return out
}

type contentEnvelope interface {
	ContentSummary() ContentSummary
	ContentData() any
	ContentMetadata() map[string]any
	ContentOutput() map[string]any
}

func NewContent[T any](summary ContentSummary, data T, metadata map[string]any, output map[string]any) *Content[T] {
	return &Content[T]{
		Summary:  summary,
		Data:     data,
		Metadata: metadata,
		Output:   output,
	}
}

func (c *Content[T]) ContentSummary() ContentSummary {
	if c == nil {
		return ContentSummary{}
	}
	return c.Summary
}

func (c *Content[T]) ContentData() any {
	if c == nil {
		return nil
	}
	return c.Data
}

func (c *Content[T]) ContentMetadata() map[string]any {
	if c == nil {
		return nil
	}
	return c.Metadata
}

func (c *Content[T]) ContentOutput() map[string]any {
	if c == nil {
		return nil
	}
	return c.Output
}

func ContentSummaryOf(content any) ContentSummary {
	if content == nil {
		return ContentSummary{}
	}
	if envelope, ok := content.(contentEnvelope); ok {
		return envelope.ContentSummary()
	}
	return ContentSummary{}
}

func ContentDataOf(content any) any {
	if content == nil {
		return nil
	}
	if envelope, ok := content.(contentEnvelope); ok {
		return envelope.ContentData()
	}
	return content
}

func ContentMetadataOf(content any) map[string]any {
	if content == nil {
		return nil
	}
	if envelope, ok := content.(contentEnvelope); ok {
		return envelope.ContentMetadata()
	}
	return nil
}

func ContentOutputOf(content any) map[string]any {
	if content == nil {
		return nil
	}
	if envelope, ok := content.(contentEnvelope); ok {
		return envelope.ContentOutput()
	}
	return nil
}

func ContentTitle(content any) string {
	return ContentSummaryOf(content).Title
}

func ContentDescription(content any) string {
	return ContentSummaryOf(content).Description
}

func ContentAuthor(content any) string {
	return ContentSummaryOf(content).Author
}

func ContentType(content any) string {
	return ContentSummaryOf(content).Type
}

func ContentAuthorNickname(content any) string {
	return ContentSummaryOf(content).AuthorNickname
}

func ContentAuthorAvatarURL(content any) string {
	return ContentSummaryOf(content).AuthorAvatarURL
}

func ContentCoverURL(content any) string {
	return ContentSummaryOf(content).CoverURL
}

func ContentDuration(content any) int64 {
	return ContentSummaryOf(content).Duration
}
