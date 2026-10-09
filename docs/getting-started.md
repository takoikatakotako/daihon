# はじめての動画

台本を書いて mp4 を出すまでの流れです。

## 1. 必要なもの

| もの | 用途 | 入れ方 |
|---|---|---|
| daihon | 本体 | 下を参照 |
| ffmpeg | 動画の書き出し | `brew install ffmpeg` |
| VOICEVOX ENGINE | 音声合成 | 下を参照 |

### daihon

[Releases](https://github.com/takoikatakotako/daihon/releases/latest) に各 OS 向けのバイナリがあります。

| OS | ファイル |
|---|---|
| macOS（Apple Silicon） | `daihon_darwin_arm64.tar.gz` |
| macOS（Intel） | `daihon_darwin_amd64.tar.gz` |
| Linux | `daihon_linux_amd64.tar.gz` / `daihon_linux_arm64.tar.gz` |
| Windows | `daihon_windows_amd64.zip` |

```bash
curl -L https://github.com/takoikatakotako/daihon/releases/latest/download/daihon_darwin_arm64.tar.gz | tar xz
sudo mv daihon /usr/local/bin/
daihon version
```

macOS でブラウザからダウンロードした場合は、署名が無いため初回起動がブロックされます。
`xattr -d com.apple.quarantine daihon` で解除してください（`curl` で取得した場合は不要です）。

Go 1.25 以降があれば、`go install` でも入ります。

```bash
go install github.com/takoikatakotako/daihon/cmd/daihon@latest
```

ソースからビルドする場合は、クローンしたディレクトリで `go install ./cmd/daihon` を実行します。

### VOICEVOX ENGINE

daihon は `localhost:50021` で待ち受けている VOICEVOX ENGINE に話しかけるだけなので、
起動のしかたは問いません。

- **Docker**: リポジトリ直下で `docker compose up -d`
- **ネイティブ版**: [リリースページ](https://github.com/VOICEVOX/voicevox_engine/releases)
  から展開し、`DAIHON_VOICEVOX_BIN=/path/to/voicevox_engine/run` を設定しておくと、
  daihon が必要なときに自動で起動し、終わったら停止します。
- **VOICEVOX アプリ**: 起動しているあいだは同じポートでエンジンが動いています。

エンジンが無くても、`--engine estimate` を付ければ文字数から尺を見積もって無音で進められます。

状態は `daihon doctor` で確認できます。

```
$ daihon doctor
✔ ffmpeg                   8.0.1 (/opt/homebrew/bin/ffmpeg)
✔ H.264 エンコーダ              h264_videotoolbox （ハードウェア）
✔ テキスト描画                   Go 側で描画（ffmpeg の drawtext/libass は使いません）
✔ VOICEVOX ENGINE          http://127.0.0.1:50021 に接続できました
```

## 2. 雛形を作る

```bash
daihon init my-video
cd my-video
```

```
my-video/
├── movie.yml            # 台本
├── characters/cast.yml  # キャラクター定義
├── assets/              # 画像・BGM
├── out/                 # 書き出し先
└── .gitignore           # out/ と .daihon-cache/ を除外
```

この時点でもうビルドできます。立ち絵が無いキャラはプレースホルダ（色付きのシルエット）で描かれます。

```bash
daihon build
```

`out/movie.mp4` ができます。

## 3. 台本を書く

`movie.yml` の `scenes` にセリフを書きます。キーがキャラクター ID、値がセリフです。

```yaml
scenes:
  - id: intro
    script:
      - a: "はじめまして。"
      - b: "台本を書くと、そのまま動画になります。"
        expr: smile
      - wait: 600
      - a: "秒数はどこにも書いていません。"
```

**秒数は書きません。** 各行の長さは合成した音声の長さで決まり、表情・画像・字幕は
すべて「行」に紐づきます。1行足せば、それ以降のタイミングは自動でずれます。

書き方の詳細は [movie.yml リファレンス](./movie-yml.md) を参照してください。

## 4. キャラクターを用意する

`characters/cast.yml` に、声と立ち絵を書きます。

```yaml
characters:
  a:
    name: "先輩"
    voice:
      speaker_id: 8          # VOICEVOX の話者 ID（春日部つむぎ）
    stand:
      position: right
    expressions:
      normal: ../assets/a/normal.png
      smile:  ../assets/a/smile.png
```

詳しくは [キャラクター](./characters.md) を参照してください。

## 5. 確認しながら作る

```bash
daihon check                             # 台本の静的検査。無い画像や未定義キャラを一覧する
daihon timeline --engine estimate -o -   # 音声を合成せずに尺と構成だけ見る
daihon build --draft                     # 960x540・半分の fps で素早く書き出す
daihon build --scene intro               # 1シーンだけ書き出す
```

音声は行ごとにキャッシュされるので、1行直して再ビルドしても合成し直すのはその行だけです。

## 6. 仕上げる

```bash
daihon build
```

ファイルを小さくしたいときは `movie.yml` に `export` を書きます
（[リファレンス](./movie-yml.md#export)）。

使った音声や素材のクレジットは `meta.credit` に書くと、動画の最後に表示されます。
VOICEVOX は話者ごとに「VOICEVOX:ずんだもん」のような表記が必要です。

## git で管理する

台本はソースコード、動画はビルド成果物として扱います。git に入れるのは
`movie.yml`・`characters/`・`assets/` で、`out/` と `.daihon-cache/` は入れません
（`daihon init` が作る `.gitignore` に書いてあります）。

配布されている立ち絵や BGM は、再配布が禁止されていることが多いので注意してください。
その場合は `assets/` を `.gitignore` に入れ、入手先を `assets/README.md` に書いておくのがおすすめです。
