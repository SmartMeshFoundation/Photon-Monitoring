# Photon Monitoring Service Online bulletin
PMS第三方监控服务`Spectrum`主网和测试网已经上线，开发者可以直接使用。下面是一些需要更改的服务配置

- 合约地址
- PFS host 
- 收费节点
- 收费token

## Spectrum 测试网
测试网PMS已经部署上线，以下是测试网更新信息

参数|参数信息
--|--
合约地址|0xa2150A4647908ab8D0135F1c4BFBB723495e8d12
PMS'IP|transport01.smartmesh.cn
PMS'端口|7004
收费节点地址|0xaed9188842c05e07bf5abdde2fb400432ae49d28
收费token|0x048257d9F5e671412E46f2Ff4B5F7AFDb7059A86




##  Spectrum 主网
主网PMS已经部署上线，以下是主网更新信息

参数|参数信息
--|--
合约地址|0x28233F8e0f8Bd049382077c6eC78bE9c2915c7D4
PMS'IP|transport01.smartmesh.cn
PMS'端口|7003
收费节点地址|0xa94399b93da31e25ab5612de8c64556694d5f2fd
收费token|0x6fdb6b4deb71c4D9AFbA4350e2e9D6CfD534F1cb



## 怎么使用？

如果你想要在主网或者测试网上使用PMS，请根据需要更新配置
- photon节点合约地址需要与PMS的合约地址一致，例如你需要在测试网上使用，请将合约地址替换为：`0xa2150A4647908ab8D0135F1c4BFBB723495e8d12`
- 调用PMS的时候，选择对应的IP，例如测试网PMS’host 应该是：http://transport01.smartmesh.cn:7004
- 具体的接口使用请参考[文档](https://photonnetwork.readthedocs.io/en/latest/sm_service/)