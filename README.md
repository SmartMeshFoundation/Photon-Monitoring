# SmartRaiden-Monitoring
SmartRaiden state channel third party monitoring service

## Usage
--address value                    The ethereum address you would like smartraiden monitoring to use sign transaction on ethereum
--keystore-path value              If you have a non-standard path for the ethereum keystore directory provide it using this argument.  (default: "/Users/bai/Library/Ethereum/keystore")
--eth-rpc-endpoint value           "host:port" address of ethereum JSON-RPC server.\n'
                                                'Also accepts a protocol prefix (ws:// or ipc channel) with optional port', (default: "/Users/bai/Library/Ethereum/geth.ipc")
--registry-contract-address value  hex encoded address of the registry contract. (default: "0xd66d3719E89358e0790636b8586b539467EDa596")
--api-port value                   port  for the RPC server to listen on. (default: 6000)
--datadir value                    Directory for storing raiden data. (default: "/Users/bai/Library/smartraidenmonitoring")
--password-file value              Text file containing password for provided account
--smt value                        smt address (default: "0x292650fee408320D888e06ed89D938294Ea42f99")