# go-load-balancer

AWS ALBの接続再利用・Timeout・切断・デプロイ時の挙動を、Goの小さな実験で理解する学習プロジェクトです。

- [TCP Echo Server](experiments/tcp-echo/README.md)：Read / WriteとTCPのバイトストリームを観察する実験

```bash
$ go run ./experiments/tcp-echo/

# 別ターミナルで
$ nc 127.0.0.1 9000
hello # 入力
hello # 返り
```
