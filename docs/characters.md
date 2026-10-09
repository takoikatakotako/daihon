# キャラクター

daihon 本体は特定のキャラクターを知りません。キャラクターは `movie.yml` か、
`imports` で読み込むファイルに書くただのデータです。

```yaml
characters:
  senpai:
    name: "先輩"                # 吹き出しの上に出る名前
    voice:
      engine: voicevox          # voicevox | estimate
      speaker_id: 8             # 話者 ID
      speed: 1.1                # 話す速さ（1.0 が標準）
      pitch: 0.0                # 声の高さ
    stand:
      position: right           # left | center | right | none
      scale: 0.72               # 立ち絵の大きさ
      offset: { x: 0, y: 40 }   # 位置の微調整（video の px 基準）
    expressions:
      smile: ../assets/senpai/smile.png
      idea:  ../assets/senpai/idea.png
    default_expression: smile
    lipsync:                    # 任意。口パク用の画像
      close: ../assets/senpai/mouth_close.png
      open:  ../assets/senpai/mouth_open.png
    bubble:
      color: "#c0507a"          # 吹き出しの色
```

キャラクター ID（上の `senpai`）が、台本でセリフを書くときのキーになります。
`show` や `wait` などの[予約語](./movie-yml.md#scenes)は ID に使えません。

## 声

`speaker_id` は VOICEVOX の話者 ID です。エンジンが起動していれば一覧を取れます。

```bash
curl -s localhost:50021/speakers | jq -r '.[] | .name as $n | .styles[] | "\(.id)\t\($n)（\(.name)）"'
```

| ID | 話者 |
|---|---|
| 2 | 四国めたん（ノーマル） |
| 3 | ずんだもん（ノーマル） |
| 8 | 春日部つむぎ（ノーマル） |
| 13 | 青山龍星（ノーマル） |

VOICEVOX の音声を使った動画には、話者ごとのクレジット（例: `VOICEVOX:ずんだもん`）が必要です。
`meta.credit` に書いておくと動画の最後に表示されます。各話者の利用規約も確認してください。

## 立ち絵

- `position` は `left` / `center` / `right`。会話なら、**質問する側を左、答える側を右**に
  置くと、字幕を読む視線の流れが自然になります。
- `position: none` にすると声だけのナレーターになります。立ち絵は描かれず、
  スライドを背景にした読み上げ動画に向いています。
- 立ち絵は下端を揃えて描かれます。高さは画面の 85% × `scale` です。
- `expressions` が空のキャラは、`bubble.color` の色のシルエットに名前と表情名を書いた
  プレースホルダで描かれます。素材が揃う前でも構成を確認できます。

## 表情

`expressions` に名前と画像を並べ、セリフの `expr:` で切り替えます。
表情は**状態**で、指定した行から次に指定するまで続きます。毎行書く必要はありません。

**1キャラの画像はすべて同じキャンバスサイズにしてください。** 表情を切り替えたときに
位置がずれないようにするためです。

## 口パク

`lipsync` に口を閉じた画像と開いた画像を書くと、音声の大きさに合わせて切り替わります。
口は表情とは別のレイヤーとして重ねるので、表情画像には口を描かず、口の画像は立ち絵と
同じキャンバスサイズの透過 PNG にします。全身の口開き・口閉じ 2 枚でも動きます。

`lipsync` を書かなければ、口パクなしで表情だけが切り替わります。

## 素材を用意する

### 生成 AI で作る

`examples/introduction` の立ち絵は ChatGPT で生成しています。プロンプトは
[`examples/introduction/PROMPTS.md`](../examples/introduction/PROMPTS.md) にあります。

- キャラごとに新しいチャットを使い、1枚目を元に「表情だけ変えて」と編集させると顔や服が揃います。
- 背景が透過ではなく、市松模様が絵として描き込まれてくることがあります。透過になっているか確認してください。

### PSDTool 形式の PSD から書き出す

配布されている立ち絵 PSD（PSDTool 形式）からは、`scripts/export-psd.py` で表情と口の PNG を
書き出せます。書き出す組み合わせはスクリプト内の `SPECS` に書きます。

```bash
pip install psd-tools
python3 scripts/export-psd.py <プロジェクト名>
```

配布素材は再配布できないことが多いので、`assets/` はリポジトリに入れず、
**素材の作り方（このスクリプト）だけをバージョン管理する**のがおすすめです。
