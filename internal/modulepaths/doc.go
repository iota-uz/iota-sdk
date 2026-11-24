// Package modulepaths holds a check that every tracked file of the module can be published: the Go
// module proxy refuses to build a version whose zip has a file path it does not accept (a Windows
// reserved name such as nul or con, an invalid character), and such a release cannot be downloaded.
package modulepaths
