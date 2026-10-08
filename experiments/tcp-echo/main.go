package main

import (
	"errors"
	"io"
	"log"
	"net"
)

func main() {
	// TCPの受付を用意する
	listener, err := net.Listen("tcp", "127.0.0.1:9000")
	if err != nil {
		log.Fatal("Listenに失敗: ", err)
	}
	defer listener.Close()

	log.Println("127.0.0.1:9000で接続を待っています")

	for {
		// クライアントからの接続を繰り返し待ち受ける
		// go run ./experiments/tcp-echo/ によりこのループに入りクライアントからの接続を待つ
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Acceptに失敗: ", err)
			return
		}
		handleConn(conn)
	}
}

// handleConn()は1つの接続が終わるまで戻らない。
// 戻るとmain()のループで再びAcceptを呼び、接続を待つ
func handleConn(conn net.Conn) {
	defer conn.Close()

	log.Println("接続: ", conn.RemoteAddr())
	defer log.Println("接続処理終了: ", conn.RemoteAddr())

	buf := make([]byte, 1024)

	for {
		// nc 127.0.0.1 9000 の実行によりクライアントからの接続受け付けるとこのループに入る
		// このループではデータの受信を待つ

		// Read は受信したデータをバッファに入れる。nは、今回実際に読み取ることが出来たバイト数
		// 例えば `hello` を5バイト読みったら、n: 5
		//
		// なぜ繰り返し内で Read するのか？
		// それは、クライアントが `hello` を送っても1回の Read で全て読める保証はない
		// 1. he → 2. llo ってこともある
		//
		// readErr: Readはnとエラーを同時に返す場合がある
		// n = 5, readErr = io.EOF
		// EOFが来たらもうこれ以上データが来ないということなので、読み取ったnを返した後に接続を閉じる
		n, readErr := conn.Read(buf)

		if n > 0 {
			// written: 実際に書き込めたバイト数
			// writeErr: 書き込み中に発生したエラー
			// buf[:n]: 先頭からnバイト分を表す
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

		// EOFなら接続を閉じる
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				log.Println("相手から受信が終了しました")
			} else {
				log.Print("Readに失敗: ", readErr)
			}
			return
		}
	}
}
