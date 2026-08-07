module tebpardaz/client

go 1.22

require (
	github.com/gorilla/websocket v1.5.3
	github.com/microsoft/go-mssqldb v1.7.2
	github.com/yaa110/go-persian-calendar v1.3.0
	gopkg.in/yaml.v3 v3.0.1
	tebpardaz/shared v0.0.0-00010101000000-000000000000
)

require (
	github.com/golang-sql/civil v0.0.0-20220223132316-b832511892a9 // indirect
	github.com/golang-sql/sqlexp v0.1.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	golang.org/x/crypto v0.18.0 // indirect
	golang.org/x/text v0.14.0 // indirect
)

replace tebpardaz/shared => ../shared

// Local clinic DB driver: microsoft/go-mssqldb (SQL Server).
// If a clinic uses another engine, swap the driver in localdb and update this require.
