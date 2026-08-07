// Package hermes provides a protocol-pluggable download task engine.
//
// New with an empty HermesNewConfig provides an in-memory Store and an HTTP
// driver for direct CreateTask use. Applications may instead provide a
// persistent Store and replace ProtocolDriver implementations. Hermes does not
// depend on API handlers, databases, or UI code.
package hermes
