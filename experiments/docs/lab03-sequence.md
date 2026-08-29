# 实验 03 时序图：OverlayFS 联合挂载

```
宿主                    OverlayFS            lower(只读)    upper(读写)
 │                        │                      │              │
 │  mkdir lower upper     │                      │              │
 │  work merged           │                      │              │
 │ ────────────────────────────────────────────>│(写入文件)    │
 │  echo lower-content >  │                      │ base.txt     │
 │  lower/base.txt        │                      │ shared.txt   │
 │                        │                      │              │
 │  mount -t overlay      │                      │              │
 │  overlay -o             │                      │              │
 │  lowerdir=...,          │                      │              │
 │  upperdir=...,          │                      │              │
 │  workdir=...            │                      │              │
 │ ──────────────────────>│                      │              │
 │                        │ (挂载到 merged)      │              │
 │                        │  merged 视图 =        │              │
 │                        │  lower ∪ upper        │              │
 │                        │                      │              │
 │  cat merged/base.txt   │                      │              │
 │  → "lower-content"     │                      │              │
 │                        │                      │              │
 │  echo modified >       │                      │              │
 │  merged/shared.txt     │                      │              │
 │ ──────────────────────>│ (CoW: 复制到 upper)  │  不变        │ 写入
 │                        │                      │              │ shared.txt
 │                        │                      │              │
 │  cat lower/shared.txt  │                      │              │
 │  → "shared-readonly"   │ (lower 未变)         │              │
 │  cat merged/shared.txt │                      │              │
 │  → "modified-in-upper" │ (upper 覆盖)          │              │
 │                        │                      │              │
 │  rm merged/base.txt    │                      │              │
 │ ──────────────────────>│ (whiteout)          │  不变        │ 创建
 │                        │                      │              │ whiteout
 │  ls merged/base.txt   │                      │              │
 │  → 不存在              │ (whiteout 隐藏)      │              │
 │  ls lower/base.txt    │                      │              │
 │  → 仍在                │ (lower 未删)          │              │
```

**安全边界**：umount 前不要删除 upper/work 目录。umount 后再 rm -rf。workdir 必须在 OverlayFS 使用的整个生命周期内存在且为空目录。
