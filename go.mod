module github.com/SmartMeshFoundation/Photon-Monitoring

replace (
	github.com/SmartMeshFoundation/Photon v0.9.3 => github.com/nkbai/Photon v1.2.0-rc6
	github.com/ethereum/go-ethereum v1.8.17 => github.com/nkbai/go-ethereum v0.1.2
	github.com/mattn/go-xmpp v0.0.1 => github.com/nkbai/go-xmpp v0.0.1
	golang.org/x/crypto v0.0.1 => github.com/golang/crypto v0.0.0-20181106171534-e4dc69e5b2fd
	golang.org/x/net v0.0.1 => github.com/golang/net v0.0.0-20181106065722-10aee1819953
	golang.org/x/sys v0.0.1 => github.com/golang/sys v0.0.0-20181106135930-3a76605856fd
	golang.org/x/tools v0.0.1 => github.com/golang/tools v0.0.0-20181106213628-e21233ffa6c3
)

require (
	github.com/SmartMeshFoundation/Photon v0.9.3
	github.com/ant0ine/go-json-rest v3.3.2+incompatible
	github.com/asdine/storm v2.1.1+incompatible
	github.com/coreos/bbolt v1.3.1-coreos.6
	github.com/ethereum/go-ethereum v1.8.17
	github.com/labstack/gommon v0.2.7
	github.com/mattn/go-colorable v0.1.0
	github.com/stretchr/testify v1.2.2
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasttemplate v0.0.0-20170224212429-dcecefd839c4 // indirect
	gopkg.in/urfave/cli.v1 v1.20.0
)
