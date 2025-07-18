# Memory Compression Example

このサンプルは、gollemの履歴圧縮機能を使用して、長い対話の中でも効率的にメモリ使用量とトークン数を管理する方法を示しています。

## 機能

- **自動履歴圧縮**: メッセージ数やトークン数の閾値を超えた時に自動的に履歴を圧縮
- **Execute内圧縮**: 1回のExecute実行中にも履歴圧縮をチェック
- **圧縮戦略**: 切り捨て、要約、ハイブリッドの3つの戦略から選択可能
- **圧縮イベント監視**: 圧縮が発生した時の詳細情報をログ出力

## 実行方法

```bash
# OpenAI APIキーを設定
export OPENAI_API_KEY="your-api-key-here"

# サンプルを実行
go run main.go
```

## 設定オプション

### CompressOptions

```go
compressOptions := gollem.CompressOptions{
    MaxMessages:       10,                       // 10メッセージで圧縮開始
    TargetTokens:      2000,                     // 2000トークンで圧縮開始  
    EmergencyTokens:   4000,                     // 4000トークンで緊急圧縮
    Strategy:          gollem.StrategyTruncate,  // 圧縮戦略
    PreserveRecent:    5,                        // 直近5メッセージを保持
    InLoopCompression: true,                     // Execute内圧縮を有効
    AggressiveMode:    false,                    // 通常モード
}
```

### 圧縮戦略

- **StrategyTruncate**: 古いメッセージを切り捨て、直近メッセージのみ保持
- **StrategySummarize**: 古いメッセージを要約してコンパクト化
- **StrategyHybrid**: 状況に応じて要約と切り捨てを使い分け

## 期待される出力

```
=== History Compression Demo ===
圧縮設定: MaxMessages=10, TargetTokens=2000, Strategy=truncate

--- 対話 1 ---
User: こんにちは！私は田中と申します。今日はよろしくお願いします。
Assistant: こんにちは田中さん！...
履歴状況: 2 メッセージ

--- 対話 2 ---
User: 私は東京在住で、プログラマーをしています。Go言語が得意です。
Assistant: プログラマーでいらっしゃるんですね！...
履歴状況: 4 メッセージ

...

🗜️  履歴圧縮が実行されました: 12 → 6 メッセージ (50.0% 削減)

--- 対話 6 ---
User: メモリ使用量を抑える方法はありますか？
Assistant: メモリ使用量を抑える方法はいくつかあります...
履歴状況: 8 メッセージ (圧縮済み, 元の長さ: 12)
```

## 利用シーン

- **長時間の対話**: カスタマーサポートや教育用チャットボット
- **リソース制約のある環境**: モバイルアプリやエッジデバイス
- **コスト最適化**: API使用料を抑えたいアプリケーション
- **複雑なツールチェーン**: 多数のツール呼び出しを伴うエージェント