Third-party licenses
====================

The comprehend.dev collector links the Go packages listed below into its binaries and
container images. Each is used under its own license, reproduced in full.

github.com/go-sql-driver/mysql is covered by the Mozilla Public License 2.0. Its source
code, in the version stated below, is available from the URL given for it.

{{ range . }}
--------------------------------------------------------------------------------
{{ .Name }}{{ with .Version }} {{ . }}{{ end }}
{{ .LicenseName }} — {{ .LicenseURL }}
--------------------------------------------------------------------------------

{{ .LicenseText }}
{{ end }}
