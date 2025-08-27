# changelog-update

GitのバージョンタグとClaudeを使用してCHANGELOG.mdを自動生成・更新するツールです。

## 概要

前のバージョンタグから現在のHEADまでの変更（差分とコミットメッセージ）を解析し、ClaudeのAIを使用してCHANGELOG.mdのエントリーを自動生成します。

## 特徴

- 📝 **[Keep a Changelog](https://keepachangelog.com/ja/1.1.0/) 形式に準拠**
- 🔄 `git pull --tags`で最新タグを自動取得（`git fetch --tags`を優先使用）
- 📊 前のタグから現在までの変更を自動検出
- 🧠 コミットメッセージと差分情報をAIで解析
- 📋 Added/Changed/Deprecated/Removed/Fixed/Security のカテゴリ自動分類
- 📚 既存のCHANGELOG.mdへの自動挿入
- 🔍 過去のタグでCHANGELOGに未記載のものを検出・追加（catch-upモード）
- ✨ **ステージングエリアの変更も含めてCHANGELOG生成**
- 🔄 **同一バージョンの既存エントリーを自動置換**（重複を防止）
- 👥 **人間にとって読みやすい形式で生成**

## インストール

### 前提条件

- [mise](https://mise.jdx.dev/)がインストールされていること
- `claude` CLIツールがインストールされていること
- Gitリポジトリ内での実行

### セットアップ

```bash
# miseでGoをインストール
mise install

# ビルド
mise build

# システムにインストール（/usr/local/bin）
mise install
```

## 使い方

### 基本的な使い方

```bash
# 新しいタグv1.0.3のCHANGELOGエントリーを生成
changelog-update --tag v1.0.3

# 過去のタグでCHANGELOGに未記載のものを追加
changelog-update --catch-up

# ビルドディレクトリから実行
./build/changelog-update --tag v1.0.3

# miseを使って実行
mise run -- --tag v1.0.3
mise changelog v1.0.3  # 短縮形
```

### オプション

```bash
--tag <version>      新しいバージョンタグ（必須）
--catch-up          CHANGELOGに未記載の過去タグを追加
--skip-pull         git pull --tagsをスキップ
--changelog <file>   CHANGELOG.mdファイルのパス（デフォルト: CHANGELOG.md）
--model <model>      使用するAIモデル（デフォルト: claude）
-m <model>           --modelの短縮形
-h, --help          ヘルプを表示
--version           バージョン情報を表示
```

## 動作フロー

### 通常モード（--tag）
1. `git pull --tags`で最新タグを取得（`git fetch --tags`を優先）
2. 最新のGitタグを検出
3. 前のタグからHEADまでの差分とコミットメッセージを取得
4. **ステージングエリアの変更も取得（git diff --cached）**
5. ClaudeのAIで変更内容を解析（コミット済み＋ステージング中の変更）
6. CHANGELOG.mdエントリーを生成（ステージング中の変更も統合して記載）
7. ユーザーの確認後、CHANGELOG.mdを更新

### catch-upモード（--catch-up）
1. `git pull --tags`で最新タグを取得（`git fetch --tags`を優先）
2. 全てのGitタグを取得
3. CHANGELOG.mdから既存のバージョンを読み取り
4. 未記載のタグを検出
5. 各タグについて変更内容を解析・エントリー生成（ステージング中の変更も含む）
6. ユーザーの確認後、CHANGELOG.mdを更新

## 生成されるCHANGELOGの形式

[Keep a Changelog](https://keepachangelog.com/ja/1.1.0/) の原則に従った形式で生成されます：

```markdown
## [v1.0.3] - 2025-08-27

### Added

- ユーザーのための新機能の説明
- プロジェクトに追加された新しい機能

### Changed

- 既存機能への変更点とその影響
- APIの変更やUIの改善など

### Deprecated

- 今後削除される予定の機能
- 非推奨となったAPIやメソッド

### Removed

- 削除された機能について
- 以前非推奨となっていた機能の削除

### Fixed

- 修正されたバグとその影響
- ユーザーが遭遇していた問題の解決

### Security

- 脆弱性の修正について
- セキュリティ関連のアップデート

```

※ 該当する変更がないセクションは表示されません。
※ 各項目は日本語で記述され、ユーザーにとって価値のある情報を重視します。

## 推奨ワークフロー

### 新しいリリースの場合
1. 開発作業を完了し、全ての変更をコミット
2. `changelog-update --tag v新バージョン` を実行
3. 生成されたCHANGELOGエントリーを確認
4. 必要に応じて手動で編集
5. CHANGELOG.mdをコミット
6. タグを作成してプッシュ

```bash
# 例
changelog-update --tag v1.0.3
# または
mise changelog v1.0.3

# CHANGELOGを確認・編集
git add CHANGELOG.md
git commit -m "docs: update changelog for v1.0.3"
git tag v1.0.3
git push && git push --tags

# またはmiseでタグ作成
mise tag v1.0.3
```

### 過去のタグを補完する場合
```bash
# CHANGELOGに未記載のタグを検出・追加
changelog-update --catch-up
# または
mise catch-up
```

## 開発

```bash
# Goバージョンの確認・インストール
mise install

# ビルド
mise build

# テスト実行
mise test

# コードフォーマット
mise fmt

# リント実行
mise lint

# ビルドアーティファクトのクリーン
mise clean

# ヘルプ表示
mise help
```

## ライセンス

MIT