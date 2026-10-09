#!/usr/bin/env python3
"""PSDTool 形式の立ち絵 PSD から、daihon が使う PNG を書き出す。

立ち絵素材は再配布条件が個別にあるためリポジトリに含められない（DESIGN.md §1.2）。
代わりにこのスクリプトを残すことで、同じ PSD さえ手元にあれば誰でも同じ素材を
再現できる。素材そのものではなく、素材の作り方をバージョン管理する。

  pip install psd-tools
  python3 scripts/export-psd.py [プロジェクト名]     # 既定: introduction

PSDTool の規約に従い、"!" 付きグループは排他選択（子をひとつだけ表示）、
レイヤー名の "*" は装飾なので比較時に無視する。

口は表情に焼き込まず、必ず別ファイルに分ける。レンダラは口を常に独立した
レイヤーとして重ねるので、表情と口の組み合わせが自由になる。
"""

import sys
from pathlib import Path

try:
    from psd_tools import PSDImage
except ImportError:
    sys.exit("psd-tools が必要です:  pip install psd-tools")

ROOT = Path(__file__).resolve().parent.parent

# 出力名 -> 排他グループごとの選択。None はそのグループ全体を隠す。
SPECS = {
    "zunda": {
        "psd": "ずんだもん立ち絵素材2.3.psd",
        "outputs": {
            # 表情には口を含めない。
            # normal は目を開けた顔（目セット = 白目 + 黒目）。なごみ目やジト目は
            # 「普通の顔」としては目を閉じている／不機嫌に見えるので既定にしない。
            # 眉は「普通眉」が波打って見えるので、まっすぐな「上がり眉」を既定にする。
            "normal": {"眉": "上がり眉", "目": "目セット", "口": None},
            # happy も目を開けたまま眉で表す。閉じ目の「にっこり」は口パクと
            # 合わせると寝ているように見える、という指摘があった。
            # 目を開けたまま normal と区別するため、顔色を「ほっぺ赤め」にする。
            "happy":  {"眉": "上がり眉", "目": "目セット", "口": None, "顔色": "ほっぺ赤め"},
            "think":  {"眉": "困り眉1", "目": "上向き",   "口": None},
            # 口だけを全画面キャンバスの透過 PNG として書き出す。
            # 閉じ口は「むー」（むにゅっとした口）ではなく、にこっとした「むふ」。
            "mouth_close": {"__only__": "口", "口": "むふ"},
            "mouth_open":  {"__only__": "口", "口": "ほあー"},
        },
    },
    "metan": {
        "psd": "四国めたん立ち絵素材2.1.psd",
        "outputs": {
            # PSD の初期状態は左腕がマイク・右腕が指差しなので、両方とも下ろす。
            "normal": {"眉": "太眉ごきげん", "目": "目セット", "口": None,
                       "右腕": "普通", "左腕": "普通"},
            "smile":  {"眉": "ごきげん",     "目": "目セット", "口": None,
                       "右腕": "普通", "左腕": "普通"},
            "mouth_close": {"__only__": "口", "口": "む"},
            "mouth_open":  {"__only__": "口", "口": "わあー"},
        },
    },
}


def plain(name: str) -> str:
    """PSDTool の装飾接頭辞を落とした名前。"""
    return name.lstrip("!*").strip()


def find_groups(psd, name: str) -> list:
    """同名グループをすべて返す。

    服装ごとに腕グループを持つ PSD（ずんだもんの 服装1 / 服装2 など）では
    同じ名前のグループが複数ある。最初に見つけたものだけを操作すると、
    表示されている服のほうが変わらないことがあるので全部に適用する。
    """
    return [l for l in psd.descendants() if l.is_group() and plain(l.name) == name]


def select(group, wanted: str | None) -> None:
    """排他グループの子を一枚だけ表示する。wanted が None ならグループごと隠す。"""
    if wanted is None:
        group.visible = False
        return
    group.visible = True
    hit = False
    for child in group:
        on = plain(child.name) == wanted
        child.visible = on
        hit = hit or on
    if not hit:
        names = ", ".join(plain(c.name) for c in group)
        raise SystemExit(f"レイヤー {wanted!r} が {plain(group.name)!r} にありません（候補: {names}）")


def export(assets: Path, character: str, spec: dict) -> None:
    src = assets / character / spec["psd"]
    if not src.exists():
        print(f"  - {src.name} がないのでスキップします")
        return

    for out_name, choices in spec["outputs"].items():
        psd = PSDImage.open(src)  # 選択が混ざらないよう毎回読み直す

        only = choices.get("__only__")
        if only:
            # 指定グループ以外を隠し、口だけの透過画像を作る。
            for layer in psd:
                layer.visible = plain(layer.name) == only

        for group_name, wanted in choices.items():
            if group_name == "__only__":
                continue
            groups = find_groups(psd, group_name)
            if not groups:
                raise SystemExit(f"グループ {group_name!r} が {src.name} にありません")
            for group in groups:
                select(group, wanted)

        dst = assets / character / f"{out_name}.png"
        psd.composite(force=True).save(dst)
        print(f"  ✔ {dst.relative_to(ROOT)}")


def main() -> None:
    project = sys.argv[1] if len(sys.argv) > 1 else "introduction"
    assets = ROOT / "examples" / project / "assets"
    if not assets.is_dir():
        sys.exit(f"{assets.relative_to(ROOT)} がありません")
    print(f"project: {project}")
    for character, spec in SPECS.items():
        print(f"{character}:")
        export(assets, character, spec)


if __name__ == "__main__":
    main()
