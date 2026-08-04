// Package hermes provides a protocol-pluggable download task engine.
//
// Hermes schedules tasks, chooses resource endpoints, persists resumable
// segments through Store, and delegates network reads to ProtocolDriver
// implementations. It does not depend on API handlers, databases, or UI code.
package hermes
