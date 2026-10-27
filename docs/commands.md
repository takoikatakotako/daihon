# コマンド

```
daihon <command> [flags]
```

| コマンド | 説明 |
|---|---|
| `init <dir>` | `movie.yml` とディレクトリの雛形を作る |
| `doctor` | ffmpeg と音声エンジンの状態を確認する |
| `check` | 台本を静的に検査する |
| `voice` | 音声だけ合成してキャッシュを温める |
| `timeline` | `timeline.json` を出力する（映像は作らない） |
| `build` | mp4 を書き出す |
| `version` | バージョンを表示する |

`daihon <command> -h` で各コマンドのフラグを表示します。

## 共通フラグ

| フラグ | 既定値 | 説明 |
|---|---|---|
| `-f <path>` | `movie.yml` | 台本のパス |
| `--engine <name>` | | 音声エンジンを上書きする（`voicevox` / `say` / `estimate`） |
| `--scene <id>` | | 指定したシーンだけを対象にする |
| `--no-autostart` | | VOICEVOX ENGINE を自動起動しない |

## init

```bash
daihon init my-video
```

`movie.yml`・`characters/cast.yml`・`.gitignore` と、`assets/`・`out/` を作ります。
立ち絵は入っていませんが、プレースホルダで描かれるのでそのままビルドできます。

## doctor

ffmpeg のバージョンと使えるエンコーダ、VOICEVOX ENGINE につながるかを確認します。
うまく動かないときに最初に実行してください。

## check

台本を読み込んで、次のようなものを一覧します。終了コードはエラーがあれば 1 です。

- エラー: 未定義のキャラクター、予約語のキャラ ID、存在しない表情、範囲外の設定値
- 警告: 存在しない画像（プレースホルダで描かれる）、長すぎるセリフ、フォント未指定、`note`

| フラグ | 説明 |
|---|---|
| `--quiet` | エラーだけ表示する |

CI で実行すると、壊れた台本を merge 前に見つけられます。

## voice

全セリフの音声を合成してキャッシュ（`.daihon-cache/audio/`）に保存します。
`build` も必要な音声を合成するので、普段は使わなくて構いません。

## timeline

台本から作った中間表現（各イベントの開始・終了時刻）を JSON で出力します。

| フラグ | 説明 |
|---|---|
| `-o <path>` | 出力先。既定は `.daihon-cache/timeline.json`、`-` で標準出力 |

```bash
daihon timeline --engine estimate -o -   # 音声エンジンなしで尺と構成を確認する
```

## build

```bash
daihon build
```

| フラグ | 説明 |
|---|---|
| `--draft` | 解像度と fps を半分にして素早く書き出す。構成やタイミングの確認用 |
| `-o <path>` | 出力先。既定は `meta.output` |

仕上げは libx264 の固定品質で書き出します。サイズや画質は `movie.yml` の
[`export`](./movie-yml.md#export) で変えられます。

## 音声エンジン

| 名前 | 説明 |
|---|---|
| `voicevox` | VOICEVOX ENGINE（`localhost:50021`）で合成する |
| `say` | macOS の音声サービスで合成する。声はキャラクターの `voice_name` で選ぶ |
| `estimate` | 合成せず、文字数から尺を見積もって無音を返す。エンジンなしで構成を確認するときや CI 向け |

| 環境変数 | 説明 |
|---|---|
| `DAIHON_VOICEVOX_BIN` | ネイティブ版エンジンの実行ファイル。必要なときに自動で起動・停止する。未設定なら VOICEVOX アプリ（`/Applications/VOICEVOX.app`）に同梱のエンジンを使う |
| `DAIHON_VOICEVOX_URL` | エンジンの URL。既定は `http://127.0.0.1:50021` |

## キャッシュ

音声は `.daihon-cache/audio/` に行単位でキャッシュされます。キーはセリフ・読み・話者・速さ・
エンジンのバージョンなどから作るので、1行直せばその行だけ合成し直されます。
消しても次のビルドで作り直されるだけです。
