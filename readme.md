## 贝壳 -- 一个基于PBFT共识算法的区块链平台

### 缘由
最初是计划学习pbft算法之后对其进行一个Go语言的实现. 编写的过程中发现, 如果想实现PBFT, 重要实现网络通信, 需要加密和验证签名功能.
于是逐渐的就写了加密模块, 区块链下载模块, 交易缓存, 数据存储等模块. 既然写了这么多, 索性就实现一个完整的区块链.


## 模块介绍
### pbft 共识
pbft共识在网络上有很多文章介绍, 但是具体的开源实现却比较少。 尤其是用在区块链项目中。 本项目的共识算法的状态流转和消息模型参考了
超级账本的sawtooth-rfcs(https://github.com/wupeaking/sawtooth-rfcs/blob/master/text/0019-pbft-consensus.md)项目的部分设计

### 加密
使用的是go标准库的椭圆曲线加密算法, 减少依赖 使用起来非常方便。
### P2P网络
在应用层封装成通信接口, 屏蔽不同的实现.
在调试场景使用http进行通信的模拟, 调试完成后使用目前比较成熟的开源libp2p进行封装.

### 交易池
一个本地的消息队列交易池 

### 虚拟机
目前只能进行账户之间的金额转账. 等此功能完全稳定后, 会考虑将自己之前实现的一个脚本解释器经过修改移植到此项目中.
[https://github.com/wupeaking/panda]

### 数据存储
定义为三级存储， 最底层为leveldb实现持久化存储。
对其他组件提供缓存层.

### 实现的功能
- pbft共识模块
    > 在单节点, 3节点, 4节点测试成功
- blockchain模块
    - 下载区块
    - 广播区块
    - 停止共识
    - 查询区块高度, 区块详情

- p2p模块
    - 封装了libp2p
    - 使用http协议进行调试通信
- 加密模块
- 存储模块
- 虚拟机模块
    - 进行转账功能
- 账户系统
    - 账户查询
    - 创建
    - 转账 
- 交易
    - 广播交易
    - 查询交易
    - 验证交易
- 命令行工具
    - 账户创建 查询 转账

## 使用示例

### 使用说明
```shell
./counch.x --help         
NAME:
   counch 贝壳-一个区块链平台 - counch --help 显示更多使用说明

USAGE:
   counch.x [global options] command [command options] [arguments...]

VERSION:
   v0.0.1

COMMANDS:
   account, account  贝壳账户系统
   help, h           Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --help, -h     show help (default: false)
   --version, -v  print the version (default: false)

```

#### 创建账户
```shell
./counch.x account create --help
NAME:
   counch 贝壳-一个区块链平台 account create - 创建一个新的账户

USAGE:
   counch 贝壳-一个区块链平台 account create [command options] [arguments...]

OPTIONS:
   --password value  密码 (default: 123456)
   --help, -h        show help (default: false)

```

#### 列出自己的账户
```shell 
./counch.x account list         
-------------------------
	address: 0x8e1fbf5b13279c82eac11cc23f456118d12a1babdecd9dbfb643defe4a1d9e62, public: 0x6a1582185f55b394b1da2e695d7935b8d38e8d30cd7e4414a5400490e6c58207f6c15ddfe41ce89a02e3716b351aea04e897bf130952b2161a4ab44c101248cc, private: , index: 1
	address: 0xf52772d71e21a42e8cd2c5987ed3bb99420fecf4c7aca797b926a8f01ea6ffd8, public: 0xc4024ffd0b42495f49002b5da606512aee341c53e43a641b7d8efac8e29f6ed2d5c6449fe4343f41c5216a84ea9dd43e07daeeadb38556bb19527ce699394cd7, private: , index: 2
累计: 2 
-------------------------


```

#### 查询当前钱包所有余额
```shell
./counch.x account balance 
```

### 如何启动一个新的贝壳链
#### 1. 编译
```
git clone https://github.com/
go build -v -o counch.x cmd/counch/main.go
```

#### 启动
```
## 创建.counch文件夹
> mkdir .counch
## 拷贝配置文件 并根据需要修改
> cp test_node1/.counch/config.json ./.counch
## 启动
> ./counch.x

```