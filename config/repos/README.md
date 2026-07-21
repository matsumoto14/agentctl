# リポジトリ設定

対象リポジトリごとの正規コマンド（lint / test / build）の正本。対象リポジトリ側には
設定ファイルを置かない（C1、設計書 §9.1）。

配置: `config/repos/<company>/<repo>.yaml`

実行時は環境変数 `AGENTCTL_CONFIG_DIR`（既定: `~/git/agentctl/config`）を起点に読む。

## スキーマ

```yaml
# ローカル clone の場所（~/ 可）
path: ~/git/myapp
# GitHub リポジトリ（owner/name）
github: owner/myapp
# PR のベースブランチ
base: main
compose:
  # 検証に使う既存 compose のサービス名
  service: app
# agentctl check が上から順に実行し、最初の失敗で打ち切る
checks:
  - name: lint
    run: [npm, run, lint]
  - name: test
    run: [npm, test]
  - name: build
    run: [npm, run, build]
```

各 check は `docker compose -p agentctl-<task> run --rm <service> <run...>` として
実行される（compose ファイルは無改変で利用する）。
