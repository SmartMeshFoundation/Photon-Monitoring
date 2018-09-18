# SmartRaiden-Monitoring
SmartRaiden Monitoring (also known as SM) is a system mainly used to facilitate the settlement process by third-party delegation when nodes are offline.

 Normally, settlement window opens after function to close payment channels is invoked during which the participant who is not responsible for closing payment channel must invoke updateTransfer & withdraw or they will lose part of money received from others. For nodes (especially mobile nodes) are not always online, there we need another trusted individual as a third-party to enforce the settlement when nodes are offline.  Nodes (as delegators) can outsource their work to the third-party SmartRaiden Monitoring service enforcing updateTransfer & withdraw on-chain to secure all the assets of nodes.


## Usage
```
USAGE:
   smartraidenmonitoring [global options] command [command options] [arguments...]

VERSION:
   0.5

COMMANDS:
     help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --address value                    The ethereum address you would like smartraiden monitoring to use sign transaction on ethereum
   --keystore-path value              If you have a non-standard path for the ethereum keystore directory provide it using this argument.  (default: "/Users/dognie/Library/Ethereum/keystore")
   --eth-rpc-endpoint value           "host:port" address of ethereum JSON-RPC server.\n'
                                                 'Also accepts a protocol prefix (ws:// or ipc channel) with optional port', (default: "/Users/dognie/Library/Ethereum/geth.ipc")
   --registry-contract-address value  hex encoded address of the registry contract. (default: "0xd66d3719E89358e0790636b8586b539467EDa596")
   --api-port value                   port  for the RPC server to listen on. (default: 6000)
   --datadir value                    Directory for storing raiden data. (default: "/Users/dognie/Library/smartraidenmonitoring")
   --password-file value              Text file containing password for provided account
   --smt value                        smt address (default: "0x292650fee408320D888e06ed89D938294Ea42f99")
   --verbosity value                  Logging verbosity: 0=silent, 1=error, 2=warn, 3=info, 4=debug, 5=trace (default: 3)
   --vmodule value                    Per-module verbosity: comma-separated list of <pattern>=<level> (e.g. eth/*=5,p2p=4)
   --backtrace value                  Request a stack trace at a specific logging statement (e.g. "block.go:271")
   --debug                            Prepends log messages with call-site location (file and line number)
   --pprof                            Enable the pprof HTTP server
   --pprofaddr value                  pprof HTTP server listening interface (default: "127.0.0.1")
   --pprofport value                  pprof HTTP server listening port (default: 6060)
   --memprofilerate value             Turn on memory profiling with the given rate (default: 524288)
   --blockprofilerate value           Turn on block profiling with the given rate (default: 0)
   --cpuprofile value                 Write CPU profile to the given file
   --trace value                      Write execution trace to the given file
   --logfile value                    redirect log to this the given file
   --help, -h                         show help
   --version, -v                      print the version
   ```
