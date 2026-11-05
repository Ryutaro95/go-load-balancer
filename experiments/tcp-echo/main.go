package main

import (
	"errors"
	"io"
	"log"
	"net"
)

func main() {
	listener, err := net.Listen("tcp", "127.0.0.1:9000")
	if err != nil {
		log.Fatal("Listenに失敗: ", err)
	}
	defer listener.Close()

	log.Println("127.0.0.1:9000で接続を待っています")

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Acceptに失敗: ", err)
			return
		}
		handleConn(conn)
	}
}

func handleConn(conn net.Conn) {
	defer conn.Close()

	log.Println("接続: ", conn.RemoteAddr())
	defer log.Println("接続処理終了: ", conn.RemoteAddr())

	buf := make([]byte, 1024)

	for {
		n, readErr := conn.Read(buf)

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
