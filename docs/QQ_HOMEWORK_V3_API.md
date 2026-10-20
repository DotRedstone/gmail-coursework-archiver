# QQ 课程插件 v3 后端契约

本文定义后端为独立 AstrBot 插件提供的接口和身份边界。服务仅保存权威花名册、提交和身份映射；QQ 事件、平台校验、角色权限及用户回复由插件负责。

## 外部身份

旧 `student_bindings.qq_id` 和 `/api/bindings/{qq_id}` 保留给兼容客户端，不解释为 QQ 官方 OpenID。新表 `external_bindings` 使用 `(platform_id, bot_id, external_user_id)` 定位平台账号，并保证新表内的 `student_id` 唯一。若旧表仍占用该学号，新自述绑定请求返回冲突，须由管理员逐条核验；不得批量转换旧 QQ 号。

`/api/v3` 接口均受 `X-API-Key` 或 Bearer 管理凭据保护：

| 方法与路径 | 行为 |
| --- | --- |
| `POST /binding-intents` | 按花名册核对学号和姓名，创建五分钟待确认状态。 |
| `GET /binding-intents` | 查询当前平台账号的待确认状态；过期返回 410。 |
| `POST /binding-intents/confirm` | 再次核对花名册并建立唯一绑定，来源记为 `self_asserted`。 |
| `POST /binding-intents/cancel` | 取消当前平台账号的待确认状态。 |
| `GET /bindings` | 按完整平台身份或学号查询；无参数列表仅供受信管理客户端使用。 |
| `POST /bindings/{student_id}/verify` | 独立核验后将绑定标记为 `staff_verified` 并记录核验者与时间。 |
| `DELETE /bindings/{student_id}` | 解除新式外部身份绑定，不修改旧绑定表。 |
| `POST /assignments/{id}/upload` | 仅接受 `staff_verified` 身份；后端依据绑定补充学生字段，并校验花名册、文件名和 ZIP 容器。 |

姓名与学号不能证明 QQ 账号归属。`self_asserted` 不能读取个人历史、附件或归档，也不能上传作业；这些操作要求 `staff_verified` 或另一独立可信核验来源。管理凭据不得交给 QQ 消息、学生或 URL。后端无法从 HTTP 参数证明 QQ 事件真实，插件必须核验平台实例、机器人、私聊来源和角色。

已有作业规则时，绑定与 `/api/roster` 使用已加载规则的名单。首份作业创建前，可在运行时数据目录提供花名册 CSV。名单缺失、损坏或内容冲突时返回服务错误，不得把名单不可用误报成学生输入错误。旧上传接口仍是管理级接口；插件须使用 v3 核验后上传接口。

## 查重与提交

合法提交即使疑似重复也照常归档，查重结果只供管理员人工核查。直接上传后更新 `plagiarism_report_cache`；查询时检查缓存版本与最新提交、文件索引是否一致，不一致则重新计算。缓存更新失败不撤销已归档提交；文件索引故障仍须运维修复。查询可从现有索引重建缓存，但不能把缓存重建称为完整查重。

## 实验卡

`POST /api/v3/lab-cards` 只接受 `实验卡N.pdf/.docx/.doc`，限制 25 MiB 并检查容器签名，原文件存入受限数据目录。上传只建立草稿和下一次周三 18:00（北京时间）的建议截止时间，不创建作业。未确认草稿可替换，已确认实验卡不可覆盖。

`POST /api/v3/lab-cards/{number}/confirm` 按数据目录中的 CSV 创建 `experiment_N` 作业规则，并要求各名单中同一学号的姓名与班级一致。当前确认结果中的 `delivery_status=manual_pending` 表示作业规则已创建，但群通知和文件尚未发送；调用方必须另行完成群投递并核验实际结果。

## 运行与迁移边界

API key 持有者拥有管理级权限。部署方应从受限凭据系统注入密钥，并对 SQLite 数据库及 WAL 做一致性备份。旧客户端和新客户端应在受控维护窗口验证兼容性；回退时按固定版本成对恢复，不把旧数字 QQ 号转换为 OpenID。
