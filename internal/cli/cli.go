// Package cli は CLI の出力形式と終了コードを定義する。
package cli

import (
	"encoding/json"
	"io"
)

// 終了コードは全コマンドで共通とし、呼び出し側が結果を判定できるようにする。
const (
	// ExitOK は成功。
	ExitOK = 0
	// ExitFailure は処理の失敗（check の fail、Agent 実行失敗など）。
	ExitFailure = 1
	// ExitUsage は使い方エラー（未知コマンド、引数不正）。
	ExitUsage = 2
	// ExitPrecondition は前提条件エラー（環境不備、lock 取得失敗など）。
	ExitPrecondition = 3
	// ExitUnimplemented は未実装スタブの暫定コード。動詞の実装が進むごとに使用箇所は消える。
	ExitUnimplemented = 10
)

// WriteJSON は v を単一の JSON オブジェクトとして w へ出力する。
// 機械処理向けの結果は stdout に、診断メッセージは stderr に書く。
func WriteJSON(w io.Writer, v any) error {
	return json.NewEncoder(w).Encode(v)
}
