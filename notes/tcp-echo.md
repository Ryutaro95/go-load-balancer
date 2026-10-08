# TCP Echo Serverの学習ノート

[main.go](../experiments/tcp-echo/main.go) のコメントを、対応するコードと一緒に整理したノート。
以下のコードは説明する箇所の抜粋。動作確認の手順は[実験用README](../experiments/tcp-echo/README.md)を参照。

## 1. TCPの受付を用意する：Listen

```go
listener, err := net.Listen("tcp", "127.0.0.1:9000")
if err != nil {
    log.Fatal("Listenに失敗: ", err)
}
defer listener.Close()
```

`net.Listen` は `127.0.0.1:9000` にTCPの受付を用意する。
戻り値の `listener` は、新しい接続を受け付けるために使う。
受付を用意できなかった場合は、`err` をログに出して終了する。

`defer listener.Close()` は、`main` から戻るときに受付を閉じる処理を予約する。
ただし、`log.Fatal` や強制終了ではdeferは実行されない。

## 2. 接続を待つ：Accept

```go
for {
    conn, err := listener.Accept()
    if err != nil {
        log.Println("Acceptに失敗: ", err)
        return
    }
    handleConn(conn)
}
```

`Accept` は、新しい接続が来るまで待つ。
接続を受け付けると、その相手と通信するための `conn` を返す。
**Listenerは接続の受付、Connは接続した相手との通信**を担当する。

サーバーを起動する：

```sh
go run ./experiments/tcp-echo/
```

別の端末から接続する：

```sh
nc 127.0.0.1 9000
```

接続を受け付けると `handleConn(conn)` が呼ばれる。
現在はこの関数を直接呼んでいるため、1つの接続の処理が終わるまで次の `Accept` へ進まない。
2つ目のクライアントが接続できても、そのデータを処理するのは1つ目の接続の処理が終わってからになる。

## 3. データを待ち、バッファに読み取る：Read

```go
buf := make([]byte, 1024)

for {
    n, readErr := conn.Read(buf)
    // この後、受信データとエラーを処理する
}
```

`buf` は受信データを入れる場所。このコードでは1024バイト分を用意する。
接続後、データがまだ届いていなければ `Read` で待つ。
`Accept` が「接続待ち」なのに対して、`Read` は「接続後のデータ待ち」になる。

| 値 | 意味 |
| --- | --- |
| `buf` | 読み取ったデータを入れる場所 |
| `n` | 今回実際に読み取ったバイト数 |
| `readErr` | 受信終了や読み取りエラーを表す値 |

例えば `hello` を5バイト読み取ったら `n == 5`。
バッファは1024バイトあっても、今回読み取った有効なデータは先頭の5バイトだけ。

### なぜReadを繰り返すのか

TCPはバイト列を運び、送信側の書き込みの区切りをReadの区切りとして保証しない。
`hello` を送っても、1回のReadで全て読めるとは限らない。

```text
送ったデータ：hello
受信の一例：1回目 he（n = 2）、2回目 llo（n = 3）
```

逆に、複数回に分けた送信を1回のReadでまとめて受け取ることもある。
そのため、送信回数とRead回数の一致を前提にせず、繰り返し読み取る。

## 4. 読み取った範囲だけを返す：Write

```go
if n > 0 {
    written, writeErr := conn.Write(buf[:n])

    if writeErr != nil {
        log.Printf(
            "Writeに失敗: %d/%dバイト書き込み済み: %v",
            written, n, writeErr,
        )
        return
    }

    if written != n {
        log.Printf("書き込み不足: %d/%dバイト", written, n)
        return
    }
}
```

`buf[:n]` は「先頭からnバイト分」を表す。
`hello` を5バイト読んだ場合、この範囲だけを送り返す。
バッファの残りには今回のデータが入っていないため、バッファ全体を送ってはいけない。

- `written`：実際に書き込めたバイト数。
- `writeErr`：書き込み中に発生したエラー。

現在のコードは、書き込みエラーがあれば書き込み済みのバイト数を記録して終了する。
続いて `written != n` も確認し、書き込み不足なら終了する。
Writeの成功は、相手のアプリケーションが読み取り・処理を終えたことまで保証するものではない。

## 5. データを扱ってから、読み取りエラーを確認する

```go
n, readErr := conn.Read(buf)

if n > 0 {
    // 前の節のWrite処理
}

if readErr != nil {
    if errors.Is(readErr, io.EOF) {
        log.Println("相手から受信が終了しました")
    } else {
        log.Print("Readに失敗: ", readErr)
    }
    return
}
```

Readの戻り値は「データかエラーか」の二者択一として扱わない。
Readerを扱う際は、`n > 0` とエラーが同時に返る場合も考える。

例えば `n == 5` と `readErr == io.EOF` が返ったと仮定すると、先にEOFだけを見て終了してしまうと、その5バイトを送り返せない。
これは戻り値の扱いを説明する例で、このTCP接続で実測した結果ではない。

このコードは**読み取ったデータを先に扱い、その後にreadErrを確認する**。
エラーがなければ次のReadへ進み、エラーがあればログを出して関数から戻る。

## 6. 接続を閉じる：deferとClose

```go
func handleConn(conn net.Conn) {
    defer conn.Close()

    // Read・Writeを繰り返す
    // EOFやエラーでreturnすると、予約したCloseが実行される
}
```

`defer` は処理を予約し、その関数を抜けるときに実行する。
`handleConn` から戻ると `conn.Close()` が実行されるため、ReadエラーでもWriteエラーでも接続を閉じる。

`io.EOF` は**相手からの受信方向の終了**を表す。
TCPには送信・受信の2方向があるため、EOFだけで両方向が終了したとは限らない。
このEcho Serverでは、EOFを確認したら関数から戻り、接続全体を閉じる方針を取っている。

接続の処理が終わると `main` に戻り、再び `Accept` で次の接続を待つ。

```text
Listen → Accept → handleConnでRead・Writeを繰り返す
                       ↓ EOFまたはエラー
                  return → ConnをClose → 次のAccept
```
