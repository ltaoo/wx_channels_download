package api

import "os"

// isMissingDownloadTaskLocalFileError reports errors which prove that a local
// file candidate cannot exist at the persisted path. Cleanup uses this instead
// of os.IsNotExist directly because some platforms return a distinct error for
// paths which cannot name a filesystem entry.
func isMissingDownloadTaskLocalFileError(err error) bool {
	return os.IsNotExist(err) || isUnaddressableDownloadTaskLocalFileError(err)
}
