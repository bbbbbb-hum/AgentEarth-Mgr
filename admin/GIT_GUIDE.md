# Git 使用指南 - API 代码更新

## 📋 常用 Git 命令

### 查看状态
```bash
git status                    # 查看当前状态
git diff                      # 查看未暂存的更改
git diff --cached             # 查看已暂存的更改
git log --oneline           # 查看提交历史
```

### 提交更改
```bash
git add .                   # 添加所有更改
git add <file>             # 添加特定文件
git commit -m "message"    # 提交更改
```

### 回滚操作
```bash
git checkout -- <file>       # 恢复文件到上次提交
git reset --soft HEAD~1      # 撤销最后一次提交，保留更改
git reset --hard HEAD~1      # 撤销最后一次提交，丢弃更改
git reset HEAD <file>        # 取消暂存文件
```

### 分支操作
```bash
git branch                   # 查看分支
git checkout <branch>        # 切换分支
git checkout -b <branch>     # 创建并切换分支
git merge <branch>          # 合并分支
```

## 🚀 API 代码更新工作流程

### 场景1：只修改了类型定义（推荐使用 update-types-git.ps1）

```bash
# 自动化脚本
.\update-types-git.ps1

# 或者手动操作
git add .
git commit -m "保存当前状态"
goctl api go -api .\admin.api -dir .\internal --style=goZero
git diff internal/logic/
git checkout HEAD~1 -- internal/logic/
git add .
git commit -m "更新 types 文件"
```

### 场景2：添加了新的 API 接口（推荐使用 update-api-git.ps1）

```bash
# 自动化脚本
.\update-api-git.ps1

# 或者手动操作
git add .
git commit -m "保存当前状态"
goctl api go -api .\admin.api -dir .\internal --style=goZero
git diff
# 检查新增的 logic 文件，手动实现业务逻辑
git add .
git commit -m "添加新的 API 接口"
```

### 场景3：修改了接口定义（参数、返回值等）

```bash
# 1. 保存当前状态
git add .
git commit -m "保存当前状态"

# 2. 重新生成代码
goctl api go -api .\admin.api -dir .\internal --style=goZero

# 3. 查看变化
git diff

# 4. 检查 logic 文件是否需要更新
git diff internal/logic/

# 5. 如果 logic 文件被修改了，需要手动更新
# 编辑 logic 文件，确保与新接口匹配

# 6. 提交更改
git add .
git commit -m "更新 API 接口"
```

## 🔧 故障排除

### 问题1：生成代码后编译失败
```bash
# 回滚到生成前的状态
git reset --hard HEAD~1

# 检查 API 定义是否有错误
goctl api validate -api .\admin.api
```

### 问题2：Logic 文件被覆盖
```bash
# 恢复 logic 文件
git checkout HEAD~1 -- internal/logic/

# 查看恢复的文件
git status

# 提交恢复的文件
git add internal/logic/
git commit -m "恢复 logic 文件"
```

### 问题3：有未提交的更改
```bash
# 暂存更改
git stash

# 更新 API 代码
.\update-types-git.ps1

# 恢复暂存的更改
git stash pop
```

## 📝 最佳实践

1. **频繁提交**：每次完成一个功能就提交
2. **清晰的提交信息**：使用有意义的提交信息
3. **使用分支**：开发新功能时使用分支
4. **定期推送**：定期推送到远程仓库
5. **使用脚本**：使用自动化脚本减少错误

## 🎯 推荐的提交信息格式

```
feat: 添加新的 API 接口
fix: 修复 API 更新后的编译错误
docs: 更新 API 文档
refactor: 重构代码结构
update: 更新 types 文件
```

## 📚 参考资料

- [Git 官方文档](https://git-scm.com/doc)
- [Go-Zero 文档](https://go-zero.dev/)
