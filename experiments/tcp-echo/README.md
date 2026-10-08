# TCP Echo Server


以下は旧課題を参考として保存したもの。現在の必須課題ではなく、必要になった時に再開する。会話では学習者の依頼により完成コードを提示済みだが、実行結果は未確認。

## 学習目標

接続の受付からRead、Write、終了までを実装する。TCPはメッセージではなくバイト列を運ぶことと、バッファ全体と読み取ったデータの違いを理解する。

## 必要な知識

- Listenerは新しい接続を待つ入口、Acceptが返すConnは1クライアントとの通信を表す。
- Readはバッファを満たすまで待つことを保証しない。返されたnが今回の有効バイト数。n > 0とerrが同時に返る可能性があるので、受信データも確認する。
- TCPでは送信側のWriteと受信側のReadの区切りが一致する保証はない。
- Writeの戻り値とエラーを検査する。短い書き込みではエラーも伴う契約があり、エラーを無視して再試行し続けない。Write成功は相手のアプリケーションが処理済みという保証ではない。
- EOFは相手からの受信方向の終了。TCPには送信・受信の2方向がある。Half-closeの本格的な実験は後のStepで行う。
- Read/Acceptは待機し得る。まずは単一接続でこの動きを観察する。Goのdeferは関数終了時に実行されるので、接続の寿命に合う位置を考える。

根拠：[Go net.Conn](https://pkg.go.dev/net#Conn)、[io.Reader](https://pkg.go.dev/io#Reader)、[io.Writer](https://pkg.go.dev/io#Writer)、[RFC 9293 §3.7](https://www.rfc-editor.org/rfc/rfc9293.html#section-3.7)。

## 実装要件

1. `experiments/tcp-echo/main.go` を学習者自身で作る。
2. `127.0.0.1:9000` でTCP接続を受け付ける。
3. 1接続から繰り返しReadし、その有効バイト列を変更せずWriteで返す。文字列や行の境界を前提にしない。最初は小さな固定長バッファでよい。
4. 書き込みバイト数・エラーを確認し、無限ループやデータ欠落を防ぐ。読み取りエラー時も取得済みデータを扱う順序を考える。
5. EOFとそれ以外のエラーを区別し、接続を確実に閉じる。1クライアントの終了後、次のクライアントを受け付ける。
6. Listen/Acceptのエラーを扱う。起動失敗は終了し、接続終了と異常のログが分かるようにする。

Day 1は逐次処理。goroutine、io.Copy、net/httpは使わず、Read/Writeを直接観察する。並行接続、期限、Graceful Shutdownは後続課題。現段階はループバック上の手動実験に限定した簡略実装。

## 動作確認方法

以下は手動確認用の手順。2026-10-09時点でCodexはサーバーやテストを実行していない。macOS版 `nc -h` で `-w`（接続と最終受信のTimeout）が利用可能なことは確認済み。

### 1. サーバーを起動する（端末A）

```sh
cd /Users/ryutaro/workdir/go-load-balancer
go run ./experiments/tcp-echo
```

期待するログは `127.0.0.1:9000で接続を待っています`。この端末は起動したままにする。停止するときは端末Aで `Ctrl+C`。現実装は終了を待つ仕組みを持たず、その時点の通信も終了する。

`address already in use` が出た場合は、別のサーバーが9000番ポートを使用していないか確認する。macOSでは `lsof -nP -iTCP:9000 -sTCP:LISTEN` で確認できる。

### 2. 対話で送受信する（端末B）

```sh
nc 127.0.0.1 9000
```

`hello` と入力してEnterを押す。一般的な端末では自分が入力した行と、サーバーから返った行の2行が表示される。端末Aには `接続:` のログが出る。

端末Bで `Ctrl+C` を押してクライアントを終了し、もう一度同じ `nc` コマンドで接続する。再び応答が返ることを確認する。サーバー側では接続処理終了のログを確認する。切断方法によりEOFまたは読み取りエラーになる場合がある。

接続後に何も入力しない間は `Read`、クライアントがいない間は `Accept` が待機する。現実装は逐次処理なので、1つ目のクライアントが接続中は2つ目のクライアントのデータを処理しない。

### 3. 改行なしのデータをバイト単位で比較する（端末B）

```sh
echo_test_dir=$(mktemp -d "${TMPDIR:-/tmp}/tcp-echo.XXXXXX")
printf 'hello' > "$echo_test_dir/input.bin"
nc -w 3 127.0.0.1 9000 < "$echo_test_dir/input.bin" > "$echo_test_dir/output.bin"
cmp "$echo_test_dir/input.bin" "$echo_test_dir/output.bin"
```

期待値：`cmp` が何も表示せず終了する（終了コード0）。差があれば差分位置が表示される。接続に失敗した場合は、サーバーの起動状態と `nc` のエラーを先に確認する。

`-w 3` はクライアント側のTimeout。入力終了後もサーバーが次のデータを待つため、受信側が待ち続けないよう付けている。サーバーのTimeout機能や送受信成功を保証するものではなく、一致の判定は `cmp` で行う。

### 4. 同じ接続で分割して送る

```sh
printf 'hello' > "$echo_test_dir/input.bin"
{ printf 'he'; sleep 1; printf 'llo'; } | nc -w 3 127.0.0.1 9000 > "$echo_test_dir/output.bin"
cmp "$echo_test_dir/input.bin" "$echo_test_dir/output.bin"
```

期待値：受信した全体が `hello` の5バイトと一致する。送信側では間隔を空けているが、サーバーのReadが必ず2回になるという保証はない。

### 5. バッファより大きいデータを送る

```sh
dd if=/dev/urandom of="$echo_test_dir/input.bin" bs=1024 count=64
nc -w 3 127.0.0.1 9000 < "$echo_test_dir/input.bin" > "$echo_test_dir/output.bin"
wc -c "$echo_test_dir/input.bin" "$echo_test_dir/output.bin"
cmp "$echo_test_dir/input.bin" "$echo_test_dir/output.bin"
```

期待値：両ファイルがそれぞれ65536バイトで、`cmp` が終了コード0となる。サーバーのバッファは1024バイトなので、繰り返しRead/Writeして全体を転送する動作を確認できる。手順3で設定した `echo_test_dir` を同じ端末で使用する。検証用ファイルは一時ディレクトリに残る。

ncのEOF後の挙動・Half-closeのオプションは実装により異なる。上記のパイプ例はmacOS向け。Linuxで使う場合は `nc -h` でオプションとEOF後の挙動を確認する。macOSの `-N` は別の意味を持つため、LinuxのHalf-close用オプションとしてそのまま使わない。tcpdump観察は別の実験で、LinuxのloとmacOSのlo0を区別する。

## 完成条件・理解度確認

上記の入出力が一致し、切断後に再接続でき、重大なレビュー指摘が解消されていること。さらに次の問いに自分の言葉で回答できること。

- 1024バイトのバッファにReadが3バイト返した場合、どの範囲を送り返すか？
- クライアントが2回Writeした時、サーバーのReadは何回になるか？
- n > 0とerrが同時に返った時、先にエラーで終了すると何が起きるか？
- ListenerとConnはなぜ別のものか？ 接続のCloseを誰が担当するか？

## 参考資料

- [Go net.Listen](https://pkg.go.dev/net#Listen)、[Listener](https://pkg.go.dev/net#Listener)、[Conn](https://pkg.go.dev/net#Conn)
- [Go io.Reader](https://pkg.go.dev/io#Reader)、[io.Writer](https://pkg.go.dev/io#Writer)
- [RFC 9293 §3.5 接続確立](https://www.rfc-editor.org/rfc/rfc9293.html#section-3.5)、[§3.6 接続終了](https://www.rfc-editor.org/rfc/rfc9293.html#section-3.6)、[§3.7 データ通信](https://www.rfc-editor.org/rfc/rfc9293.html#section-3.7)

RFCを通読してから始める必要はない。まずAPIと実測を結び付け、必要な節を読む。
