package aico

import "fmt"

// CreateOpenAIQuestion formats a question for the OpenAI API based on the git diff output.
func CreateOpenAIQuestion(diffOutput string, numCandidates int, japaneseOutput bool) string {
	prompt := `
Please generate %d appropriate commit message candidates based on the context.
(Do NOT number at the beginning of the line)

Here is a sample of commit messages for different scenarios.

---

# Adding a new feature
Add search functionality to homepage

# Bug fix
Fix bug causing app crash on login

# Code refactoring
Refactor data parsing function for readability

# Adding a test
Add unit tests for user registration

# Document update
Update README with new installation instructions

# Performance improvement
Improve loading speed of product images

# Dependency update
Update lodash to version 4.17.21

# Removing unnecessary code
Remove deprecated API endpoints

# UI/UX enhancement
Enhance user interface for mobile view

# Adding or modifying code comments
Update comments in the routing module
---

Output Format:
- Add diff loader module for handling Git diffs
- Implement diff loading from file and Git in diffloader.ts
- Create diffloader.ts to process and split Git diffs

---

%s`
	if japaneseOutput {
		prompt = `
以下の内容に基づいて、%d個の適切なコミットメッセージ候補を生成してください。
（行の先頭に番号を付けないでください）

以下は、さまざまなシナリオのコミットメッセージのサンプルです。

---

# 新機能の追加
ホームページに検索機能を追加

# バグ修正
ログイン時にアプリがクラッシュするバグを修正

# コードのリファクタリング
可読性のためにデータ解析関数をリファクタリング

# テストの追加
ユーザー登録の単体テストを追加

# ドキュメントの更新
新しいインストール手順でREADMEを更新

# パフォーマンスの向上
製品画像の読み込み速度を向上

# 依存関係の更新
lodashをバージョン4.17.21に更新

# 不要なコードの削除
廃止されたAPIエンドポイントを削除

# UI/UXの改善
モバイルビューのユーザーインターフェースを改善

# コードコメントの追加または変更
ルーティングモジュールのコメントを更新
---

出力形式:
- Gitの差分を処理するためのdiffローダーモジュールを追加
- diffloader.tsでファイルとGitからの差分の読み込みを実装
- Gitの差分を処理し、分割するためのdiffloader.tsを作成

---

%s`
	}
	return fmt.Sprintf(prompt, numCandidates, diffOutput)
}
