# AutoOps 天枢前端 UI 设计规范

> 版本：1.0  
> 更新日期：2026-07-24  
> 适用技术栈：Vue 3、Element Plus、ECharts  
> 视觉参考：`src/views/dashboard/Dashboard.vue`、`src/views/dashboard/systemd-dashboard.vue`

## 1. 文档目标

本规范用于统一 AutoOps 天枢中资产、告警、监控、任务、系统服务、AI 分析等运维页面的视觉语言与交互行为。目标不是让所有页面长得完全一样，而是让用户在不同模块中始终感受到同一套产品秩序。

新页面应直接按本规范实现；旧页面改造时，应优先统一页面容器、卡片、按钮、弹框和表单，再处理局部业务样式。

## 2. 核心设计原则

### 2.1 高信息密度，但不拥挤

- 运维产品以扫描、比较、定位问题为主，首屏应尽量呈现有效数据。
- 使用 `8px` 左右的模块间距和紧凑标题栏，避免大面积无信息空白。
- 信息密度来自合理分组，不来自压缩到难以阅读。
- 页面主标题、卡片标题、数据值、辅助说明必须形成清晰层级。

### 2.2 浅色工作台，状态色负责强调

- 页面背景保持浅灰蓝或透明，由主框架承载整体背景。
- 主内容面以白色、`#fbfdff` 和极浅蓝为主。
- 蓝色用于主操作和信息状态；绿色、橙色、红色仅表达业务状态。
- 不用大面积高饱和背景，不让装饰色压过数据。

### 2.3 克制的圆角和阴影

- 卡片圆角用于建立层级，不将所有元素做成胶囊形。
- 阴影只提供轻微悬浮感，边框仍是工作台页面的主要分隔方式。
- 胶囊圆角仅用于标签、状态徽标、排名编号等小型元素。

### 2.4 一致的组件状态

- 可点击元素必须具备默认、悬停、聚焦、按下、禁用和加载状态。
- 状态不能只依靠颜色表达，应配合图标、文字或形状。
- 动效以 `150ms - 200ms` 为主，只表现反馈，不做持续装饰动画。

## 3. 页面结构

推荐页面结构如下：

```text
页面容器
├── 紧凑页头：标题、上下文、核心操作
├── 核心指标：4 - 7 个数据卡片
├── 业务分区卡片
│   ├── 分区标题栏
│   └── 图表、列表、表格或表单
└── 次级业务分区
```

### 3.1 页面容器

| 属性 | 标准值 | 说明 |
| --- | --- | --- |
| 主内容区外边距 | `10px` | 必须显式设置或通过计算样式确认 `.el-main` 已提供 |
| 页面内边距 | `7px` | 大盘和工作台页面 |
| 页面模块间距 | `8px` | 纵向主布局 |
| 普通业务页内边距 | `12px - 16px` | 非大盘页面可适当放宽 |
| 页面高度 | `calc(100vh - 124px)` | 当前主框架下的工作区高度 |
| 页面滚动 | 页面容器内部滚动 | 防止整个应用产生双滚动条 |

```css
.ops-page {
  box-sizing: border-box;
  height: calc(100vh - 124px);
  min-height: 0;
  overflow: auto;
  padding: 7px;
  color: var(--ops-text-primary);
  background: transparent;
}

.ops-workbench {
  display: grid;
  gap: 8px;
  min-width: 0;
}
```

#### 3.1.1 主框架与页面双层间距（强制）

页面边距由两层共同组成，二者不能混淆，也不能只实现其中一层：

```text
.el-main 主框架边距 10px
└── 页面容器 padding 7px
    └── 页头或主卡片
```

桌面端页头或主卡片边框距离 `.el-main` 可视边缘应为 `17px`：`10px` 主框架边距加 `7px` 页面内边距。卡片自身的内容 padding 不计入这 `17px`。

当前 `Home.vue` 只统一了 `.el-main` 的背景、底部空间和滚动方式，不能假设 Element Plus 默认提供的顶部、左侧、右侧 padding 等于 `10px`。每个独立工作台页面必须使用页面根类显式约束，或在主框架完成全局统一后验证计算值确实为 `10px`。

```css
/* 将 account-page 替换为当前页面唯一的根类 */
:global(.home-container .el-main:has(.account-page)) {
  padding-top: var(--ops-shell-gutter, 10px);
  padding-right: var(--ops-shell-gutter, 10px);
  padding-left: var(--ops-shell-gutter, 10px);
}

.account-page {
  box-sizing: border-box;
  padding: var(--ops-page-padding, 7px);
}
```

强制要求：

- 不得因为 `.el-main` “看起来已有间距”而省略计算样式检查。
- 不得用页面根容器的 `17px` padding 代替双层结构，否则页面高度、滚动和跨模块布局会不一致。
- 不得重复设置两层 `10px`，避免卡片距离边缘扩大为 `20px` 或更多。
- `.el-main` 的底部 padding 由主框架页脚策略控制，业务页面不得为了对齐左右边距随意覆盖。
- 新页面和旧页面改造完成后，必须在浏览器计算样式中确认 `.el-main` 顶部、左侧、右侧均为 `10px`，页面根容器 padding 为 `7px`。
- 桌面端最终验收值：页头或第一张主卡片边框到 `.el-main` 顶部、左侧、右侧的距离均为 `17px`。存在滚动条时，右侧以内容盒可视边缘为准。

### 3.2 紧凑页头

大盘页头不是宣传页 Hero，不使用超大标题和大段描述。

| 元素 | 标准值 |
| --- | --- |
| 页头内边距 | `8px 12px` |
| 页头圆角 | `12px` |
| 图标显示区 | `42px × 42px`，不设置边框和有色外框 |
| 业务图标 | `28px - 30px`，保持原始比例 |
| 主标题 | `20px / 1.2 / 700` |
| 标题与说明 | 桌面端同行，间距 `10px`，中间使用 `1px × 22px` 分隔线 |
| 说明文字 | `12px / 1.5 / 400`，默认单行省略 |
| 操作区间距 | `8px` |

资产管理和普通工作台页头采用“业务图标 + 标题 + 分隔线 + 一句话说明”的横向结构。图标不再放进带边框的小卡片中，避免形成多余的嵌套层级；业务图标本身应足够清晰，不能依赖边框强调。

说明文字只补充当前页面的管理范围或业务上下文，桌面端保持单行并在空间不足时省略。`768px` 以下允许标题与说明改为上下排列，同时隐藏分隔线。没有有效说明时可以省略说明和分隔线，但不得保留空占位。

```vue
<header class="ops-page-header">
  <span class="ops-page-header__icon" aria-hidden="true">
    <img src="@/assets/image/业务图标.svg" alt="">
  </span>
  <div class="ops-page-header__content">
    <h1>页面标题</h1>
    <span class="ops-page-header__divider" aria-hidden="true"></span>
    <p>一句话说明当前页面管理的对象与范围。</p>
  </div>
</header>
```

```css
.ops-page-header__icon {
  display: grid;
  width: 42px;
  height: 42px;
  flex: 0 0 42px;
  place-items: center;
  border: 0;
  background: transparent;
}

.ops-page-header__icon img {
  width: 30px;
  height: 30px;
  object-fit: contain;
}

.ops-page-header__content {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.ops-page-header__divider {
  width: 1px;
  height: 22px;
  flex: 0 0 1px;
  background: var(--ops-border-soft);
}

.ops-page-header__content p {
  margin: 0;
  overflow: hidden;
  color: var(--ops-text-muted);
  font-size: 12px;
  line-height: 1.5;
  text-overflow: ellipsis;
  white-space: nowrap;
}
```

### 3.3 栅格布局

| 场景 | 大屏 | 中屏 | 小屏 |
| --- | --- | --- | --- |
| 核心指标卡 | 7 列 | 4 列 | 2 列 / 1 列 |
| 资产分布图表 | 4 列 | 2 列 | 1 列 |
| 告警态势卡片 | 4 列 | 2 列 | 1 列 |
| 普通表单 | 3 列 | 2 列 | 1 列 |
| 弹框复杂表单 | 2 列 | 1 列 | 1 列 |

推荐断点：

```css
@media (max-width: 1500px) { /* 4 列降为 2 列 */ }
@media (max-width: 1200px) { /* 复杂左右布局改为单列 */ }
@media (max-width: 960px)  { /* 图表和表单改为单列 */ }
@media (max-width: 620px)  { /* 指标卡改为单列，页面 padding 6px */ }
```

所有 Grid 子项必须设置 `min-width: 0`，避免长文本、表格或图表撑破布局。

## 4. 设计令牌

项目应逐步使用三层令牌：基础值、语义值、组件值。业务组件优先引用语义令牌或组件令牌，不直接散落硬编码颜色。

### 4.1 基础颜色

| 令牌 | 值 | 用途 |
| --- | --- | --- |
| `--ops-blue-500` | `#409eff` | Element Plus 主色、聚焦 |
| `--ops-blue-600` | `#2563eb` | 主操作、图标、关键信息 |
| `--ops-cyan-500` | `#0ea5e9` | 基础设施、网络信息 |
| `--ops-green-500` | `#10b981` | 正常、成功、在线 |
| `--ops-orange-500` | `#f59e0b` | 警告、待处理、风险 |
| `--ops-red-500` | `#ef4444` | 严重、失败、危险 |
| `--ops-purple-500` | `#6366f1` | 分类、分析、辅助强调 |
| `--ops-teal-500` | `#14b8a6` | AI 工具、连接成功 |
| `--ops-slate-950` | `#0f172a` | 一级标题、关键数据 |
| `--ops-slate-800` | `#172033` | 正文主色 |
| `--ops-slate-600` | `#53657d` | 标签、次级正文 |
| `--ops-slate-500` | `#64748b` | 辅助说明 |
| `--ops-slate-400` | `#7b8aa0` | 弱提示、时间 |

### 4.2 背景和边框

| 令牌 | 值 | 用途 |
| --- | --- | --- |
| `--ops-bg-page` | `#f6f8fb` | 独立业务页背景 |
| `--ops-bg-surface` | `#ffffff` | 主卡片、弹框 |
| `--ops-bg-surface-soft` | `#fbfdff` | 图表卡、表单控件 |
| `--ops-bg-subtle` | `#f3f7fc` | 表头、筛选条、次级区块 |
| `--ops-border` | `#dbe7f4` | 主卡片边框 |
| `--ops-border-soft` | `#e7eef7` | 标题栏和内容分隔线 |
| `--ops-border-inner` | `#dce8f5` | 内部卡片边框 |
| `--ops-border-hover` | `#b9d4ff` | 悬停和可交互提示 |

### 4.3 间距

以 `4px` 为基础单位，项目常用值如下：

| 令牌 | 值 | 用途 |
| --- | --- | --- |
| `--ops-space-1` | `4px` | 微间距 |
| `--ops-space-1-5` | `6px` | 图标与文字、紧凑列表 |
| `--ops-space-2` | `8px` | 默认网格间距、按钮组 |
| `--ops-space-2-5` | `10px` | 内部卡片 padding |
| `--ops-space-3` | `12px` | 标题栏、普通内容 |
| `--ops-space-3-5` | `14px` | 表单列间距 |
| `--ops-space-4` | `16px` | 弹框和宽松卡片 |
| `--ops-space-6` | `24px` | 弹框复杂列间距 |

禁止在同一模块中混用无规律的 `11px`、`13px`、`17px` 间距。特殊值必须由组件结构明确要求。

### 4.4 圆角

| 令牌 | 值 | 使用范围 |
| --- | --- | --- |
| `--ops-radius-sm` | `7px` | 紧凑输入框、筛选控件 |
| `--ops-radius-md` | `8px` | 按钮、表格、普通输入框 |
| `--ops-radius-inner` | `10px` | 指标卡、图表卡、内部面板 |
| `--ops-radius-card` | `12px` | 页面主卡片、页头 |
| `--ops-radius-dialog` | `15px` | 普通表单弹框，统一标准 |
| `--ops-radius-ai` | `20px` | AI 助手强调型弹框专用 |
| `--ops-radius-full` | `999px` | 标签、状态点、排名编号 |

圆角使用规则：

- 普通页面卡片不超过 `12px`。
- 普通弹框固定为 `15px`，不要在不同弹框中任意使用 `8px`、`12px`、`18px`。
- `20px` 仅用于 AI 助手这类独立、沉浸式组件。
- 文字按钮和菜单项不使用胶囊圆角。

### 4.5 阴影

| 令牌 | 值 | 使用范围 |
| --- | --- | --- |
| `--ops-shadow-card` | `0 8px 24px rgba(80, 124, 180, 0.07)` | 主卡片 |
| `--ops-shadow-card-hover` | `0 10px 24px rgba(64, 158, 255, 0.10)` | 可点击卡悬停 |
| `--ops-shadow-hero` | `0 10px 28px rgba(80, 124, 180, 0.10)` | 紧凑页头 |
| `--ops-shadow-dialog` | `0 22px 60px rgba(49, 92, 142, 0.18)` | 普通弹框 |
| `--ops-shadow-dialog-strong` | `0 28px 80px rgba(39, 70, 110, 0.24)` | AI 助手 |

卡片同时使用浅边框和弱阴影。禁止使用大面积黑色重阴影或多层发光效果。

### 4.6 字体层级

项目使用系统无衬线字体。数字、IP、端口、百分比和代码使用等宽字体。

| 场景 | 字号 | 字重 | 行高 |
| --- | --- | --- | --- |
| 页面主标题 | `20px` | `700` | `1.2` |
| 弹框标题 | `17px - 18px` | `600 - 700` | `1.3` |
| 分区标题 | `15px - 16px` | `700` | `1.2 - 1.25` |
| 内部卡片标题 | `13px` | `700 - 800` | `1.35` |
| 正文 | `13px` | `400` | `1.5 - 1.7` |
| 表单标签 | `12px` | `700` | `1.4` |
| 表格、辅助文字 | `11px - 12px` | `400 - 600` | `1.35 - 1.5` |
| 指标值 | `17px - 26px` | `700 - 800` | `1.0 - 1.15` |
| AI 评分等重点数字 | `38px` | `700` | `1` |

所有字距保持 `letter-spacing: 0`。不要随视口宽度缩放字号。

### 4.7 推荐令牌代码

```css
:root {
  /* Primitive */
  --ops-blue-500: #409eff;
  --ops-blue-600: #2563eb;
  --ops-cyan-500: #0ea5e9;
  --ops-green-500: #10b981;
  --ops-orange-500: #f59e0b;
  --ops-red-500: #ef4444;
  --ops-purple-500: #6366f1;
  --ops-teal-500: #14b8a6;

  /* Semantic */
  --ops-text-heading: #0f172a;
  --ops-text-primary: #172033;
  --ops-text-secondary: #53657d;
  --ops-text-muted: #64748b;
  --ops-color-primary: var(--ops-blue-600);
  --ops-color-success: var(--ops-green-500);
  --ops-color-warning: var(--ops-orange-500);
  --ops-color-danger: var(--ops-red-500);
  --ops-bg-page: #f6f8fb;
  --ops-bg-surface: #fff;
  --ops-bg-surface-soft: #fbfdff;
  --ops-border: #dbe7f4;
  --ops-border-soft: #e7eef7;

  /* Component */
  --ops-card-radius: 12px;
  --ops-card-padding: 12px;
  --ops-card-shadow: 0 8px 24px rgba(80, 124, 180, 0.07);
  --ops-shell-gutter: 10px;
  --ops-page-padding: 7px;
  --ops-module-gap: 8px;
  --ops-button-radius: 8px;
  --ops-input-radius: 8px;
  --ops-dialog-radius: 15px;
  --ops-control-height: 34px;
  --ops-transition-fast: 160ms ease;
}
```

## 5. 卡片规范

### 5.1 卡片层级

| 类型 | 圆角 | 内边距 | 背景 | 使用场景 |
| --- | --- | --- | --- | --- |
| 页面主卡片 | `12px` | 标题 `8px 12px`，内容 `8px 10px 10px` | 白色 | 一个完整业务分区 |
| 指标卡 | `10px` | `8px 11px` | 白色渐变 | 核心数字 |
| 图表卡 | `10px` | `8px 10px 6px` | `#fbfdff` | 单个 ECharts 图表 |
| 列表项 | `8px - 9px` | `4px 7px` 或 `8px 10px` | 白色 | 告警、资源、任务行 |
| 辅助面板 | `10px - 12px` | `10px - 13px` | `#f8fbff` | 弹框说明、规则提示 |

### 5.2 主卡片

```css
.ops-card {
  min-width: 0;
  overflow: hidden;
  border: 1px solid var(--ops-border);
  border-radius: var(--ops-card-radius);
  background: rgba(255, 255, 255, 0.96);
  box-shadow: var(--ops-card-shadow);
}

.ops-card__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 12px;
  border-bottom: 1px solid var(--ops-border-soft);
}
```

不要把页面每个区块都做成浮动卡片。主卡片用于完整业务分区，内部只允许出现数据卡、图表卡、列表项等有明确功能的二级容器，避免“卡片套卡片套卡片”。

### 5.3 指标卡

- 最小高度：`78px`。
- 图标区：`34px × 34px`，圆角 `10px`。
- 标题：`10.5px - 12px`，单行省略。
- 数值：`17px` 等宽字体，可换行但不能溢出。
- 说明：`11px`，最多两行。
- 可点击卡悬停时上移 `1px`，边框变为 `#b9d4ff`。
- 顶部 `3px` 状态色可用于区分指标类别，但一组卡片中颜色应有明确语义。

### 5.4 图表卡

- 使用 `3px` 顶部色条区分类别。
- 标题行图标为 `24px × 24px`，圆角 `8px`。
- 紧凑图表高度为 `160px`；主要资产图表为 `172px - 210px`。
- 图表容器必须有稳定高度，加载态和空状态不能导致布局跳动。
- 图例优先置底，数量多时启用滚动，不挤压绘图区。

## 6. 按钮规范

### 6.1 尺寸

| 尺寸 | 高度 | 水平内边距 | 字号 | 图标 |
| --- | --- | --- | --- | --- |
| 小型 | `28px - 30px` | `10px` | `12px` | `14px` |
| 默认 | `32px - 34px` | `12px - 14px` | `13px - 14px` | `16px` |
| 大型 | `38px - 40px` | `16px - 18px` | `14px` | `18px` |
| 纯图标 | 与同级按钮等高等宽 | `0` | - | `16px - 18px` |

默认圆角统一为 `8px`。只有标签、状态筛选项可使用胶囊形；常规命令按钮不使用胶囊形。

高密度表格筛选工具栏中，如果同一区域连续出现 `3` 个或以上命令按钮，按钮强制采用小型规格：高度 `30px`、水平内边距 `10px`、字号 `12px`、图标 `14px`。同一工具栏中的输入框和下拉框仍保持 `34px`，不得使用页面级通用 `.el-button { min-height: 34px; }` 覆盖小型按钮。

### 6.2 按钮类型

| 类型 | 视觉 | 用途 |
| --- | --- | --- |
| 主按钮 | 蓝色或蓝色渐变、白字 | 每个区域唯一的主要操作 |
| 次按钮 | 白底、浅蓝灰边框、深灰字 | 取消、返回、次级操作 |
| 文字按钮 | 透明背景、主色文字 | 表格行内轻量操作 |
| 危险按钮 | 红色、白字 | 删除、停止、不可逆操作 |
| 警告按钮 | 橙色或浅橙底 | 需要注意但可恢复的操作 |
| 图标按钮 | 方形、仅图标、带 Tooltip | 刷新、关闭、折叠、复制 |

### 6.3 工作台工具栏按钮颜色

资产、告警、监控、任务等工作台页面的工具栏按钮，按操作含义固定颜色。此规则优先于通用按钮类型，不允许同一种操作在不同页面随意换色。

| 操作语义 | 标准颜色 | Element Plus 类型 | 示例 |
| --- | --- | --- | --- |
| 查询 | 蓝色 `#409eff` | `type="primary"` | 查询、搜索、检索 |
| 刷新 | 浅黄色 `#f3c66b` | `type="warning"` + 刷新专用类 | 刷新、重新加载 |
| 新建 | 绿色 `#67c23a` | `type="success"` | 新建、创建、添加 |
| 导出 | 绿色 `#67c23a` | `type="success"` | 导出当前列表、导出选中 |
| 静默与规则管理 | 蓝色 `#409eff` | `type="primary"` | 静默规则、规则管理 |
| 重置 | 黄色 `#e6a23c` | `type="warning"` | 重置、清空筛选 |
| 其他工具命令 | 黄色 `#e6a23c` | `type="warning"` | 导入、批量操作等 |

工具栏按钮必须使用“实色背景 + 白色文字”。带图标时，图标使用白色填充或继承白色文字颜色，不使用白底描边、浅色 Plain 或透明 Text 样式。

```vue
<el-button type="primary"><el-icon><Search /></el-icon>查询</el-button>
<el-button class="ops-refresh-button" type="warning"><el-icon><Refresh /></el-icon>刷新</el-button>
<el-button type="warning"><el-icon><RefreshRight /></el-icon>重置</el-button>
<el-button type="success"><el-icon><Plus /></el-icon>新建</el-button>
```

```css
.toolbar .el-button--primary {
  color: #fff;
  border-color: #409eff;
  background: #409eff;
}

.toolbar .el-button--success {
  color: #fff;
  border-color: #67c23a;
  background: #67c23a;
}

.toolbar .ops-refresh-button {
  color: #fff;
  border-color: #f3c66b;
  background: #f3c66b;
}

.toolbar .el-button--warning {
  color: #fff;
  border-color: #e6a23c;
  background: #e6a23c;
}

.toolbar .el-button .el-icon {
  color: inherit;
}
```

以下场景不套用“其他按钮为黄色”的规则：

- 删除、停止等不可逆操作保持红色危险样式。
- 弹框中的保存、确认保持蓝色主按钮，取消保持中性次按钮。
- 表格行内编辑、查看、删除可继续使用文字按钮，避免高密度表格出现大量色块。
- 关闭、折叠等无文字图标按钮按所在组件语义处理。

### 6.4 交互状态

| 状态 | 规范 |
| --- | --- |
| Hover | 边框或背景加深，阴影轻微增强，最多上移 `1px` |
| Active | 回到 `translateY(0)`，颜色再加深一级 |
| Focus-visible | `2px` 主色聚焦环，键盘操作必须可见 |
| Disabled | 禁止点击，透明度约 `0.5`，不保留悬停动效 |
| Loading | 保持原宽度，显示加载图标并阻止重复提交 |

新页面不建议使用全局 `.el-button { transition: all }`；应只过渡颜色、边框、阴影和位移，避免宽高等属性参与动画。

```css
.ops-button {
  min-height: 32px;
  border-radius: 8px;
  font-weight: 600;
  transition:
    color 160ms ease,
    background-color 160ms ease,
    border-color 160ms ease,
    box-shadow 160ms ease,
    transform 160ms ease;
}

.ops-button:hover {
  transform: translateY(-1px);
}

.ops-button:active {
  transform: translateY(0);
}
```

### 6.5 渐变使用

渐变用于重要操作、AI 能力和弹框强调，不用于所有按钮。

| 名称 | 渐变 | 场景 |
| --- | --- | --- |
| 主操作蓝 | `linear-gradient(135deg, #5aa8ff, #2563eb)` | 弹框保存、确认、执行 |
| 暖色提示 | `linear-gradient(135deg, #ffd36a, #f59e0b)` | 风险确认、AI 建议 |
| 暖到冷品牌渐变 | `linear-gradient(90deg, #f5a524 0%, #62a8ff 48%, #55d6be 100%)` | AI 助手顶部细线、强调边框 |
| 暖到冷浅背景 | `linear-gradient(90deg, #fff6e7 0%, #f7fbff 36%, #ecf7ff 100%)` | AI 弹框标题区 |

暖到冷渐变应作为细线、边框或浅背景使用。不要把橙、蓝、绿色高饱和渐变铺满整个工作台。

## 7. 弹框规范

### 7.1 类型和尺寸

| 类型 | 推荐宽度 | 圆角 | 用途 |
| --- | --- | --- | --- |
| 确认弹框 | `384px - 480px` | `15px` | 删除、停止、确认 |
| 标准表单 | `620px - 760px` | `15px` | 创建或编辑单一对象 |
| 复杂表单 | `840px - 980px` | `15px` | 多列表单、带辅助面板 |
| AI 助手 | 依据工作区，最大 `calc(100vw - 64px)` | `20px` | 对话、推理、分析 |

移动端弹框宽度使用 `calc(100% - 24px)`，内容过长时仅让 Body 滚动，标题和底部操作保持可见。

### 7.2 普通表单弹框

| 区域 | 标准值 |
| --- | --- |
| 外框 | `1px solid #d7e6f6` |
| 圆角 | `15px` |
| Header padding | `16px 18px 12px` |
| Body padding | `16px 18px` |
| Footer padding | `12px 18px 16px` |
| Header / Footer 分隔线 | `#e3edf8` |
| 标题图标 | `38px × 38px`，圆角 `12px` |
| 标题 | `17px / 700` |
| 副标题 | `12px` |
| Footer 按钮间距 | `10px` |

复杂弹框允许使用“主表单 + 右侧辅助说明”布局：

```css
.dialog-body-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 230px;
  gap: 14px;
}

.dialog-insight {
  padding: 13px;
  border: 1px solid #cfe2ff;
  border-radius: 12px;
  background: #f6faff;
}
```

右侧辅助面板只放校验说明、影响范围、风险提示或实时预览，不重复描述用户已经能从表单标签理解的内容。

### 7.3 Element Plus 弹框样式作用域

`el-dialog` 默认通过 Teleport 挂载到 `body`，不在当前 Vue 组件的 scoped DOM 内。因此仅写：

```css
/* 错误：在 scoped 样式中通常无法命中 Teleport 后的弹框 */
.my-dialog .el-dialog {
  border-radius: 15px;
}
```

可能不会生效。统一使用 `modal-class` 定位 Overlay，并通过 `:global()` 设置弹框：

```vue
<el-dialog
  v-model="visible"
  class="ops-dialog"
  modal-class="ops-form-dialog-overlay"
>
  ...
</el-dialog>
```

```css
:global(.ops-form-dialog-overlay .el-dialog) {
  --el-dialog-border-radius: 15px;
  overflow: hidden;
  border-radius: var(--el-dialog-border-radius) !important;
}

:global(.ops-dialog .el-dialog__header) {
  padding: 16px 18px 12px;
  border-bottom: 1px solid #e3edf8;
}
```

如果圆角仍然看不出来，应依次检查：

1. `modal-class` 是否加在当前弹框上。
2. 选择器是否命中 Teleport 后的 `.el-dialog`。
3. `.el-dialog` 是否设置 `overflow: hidden`，避免内部背景覆盖圆角。
4. Element Plus 的 CSS 变量或内联样式是否覆盖 `border-radius`。
5. 修改的是否是实际打开的弹框，而不是同文件中的另一个弹框。

### 7.4 AI 助手弹框

AI 助手是强调型例外，可以使用更强的品牌层次：

- 外框圆角 `20px`。
- 暖到冷渐变仅作为 `1px` 边框和顶部 `3px` 色带。
- 顶栏使用低饱和暖到冷浅背景。
- 对话区保持白色和浅蓝色，不使用深色霓虹背景。
- 用户消息靠右，圆角 `16px 16px 4px 16px`。
- 推理块和答案块圆角 `16px`，边框 `#cfe2f8`。
- 风险推理块使用浅暖色背景与橙色边框。
- 工具调用标签可使用胶囊形，但正文容器不可胶囊化。

## 8. 表单规范

### 8.1 表单布局

- 页面表单默认使用顶部标签，便于不同长度字段在栅格中对齐。
- 紧凑后台表单允许使用左侧标签，标签宽度建议 `88px - 104px`。
- 页面表单最多 3 列；弹框表单最多 2 列。
- 有依赖关系的字段放在同一行或同一分组。
- 说明、脚本、PromQL、日志查询等长内容字段必须独占整行。

```css
.ops-form {
  --el-component-size: 34px;
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px 14px;
}

.form-grid.two-col {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.form-field-full {
  grid-column: 1 / -1;
}
```

### 8.2 标签和必填项

| 属性 | 标准值 |
| --- | --- |
| 标签字号 | `12px` |
| 标签字重 | `700` |
| 标签颜色 | `#334155` 或 `#475569` |
| 标签底部间距 | `6px` |
| 表单项底部间距 | `12px - 14px` |
| 必填星号 | Element Plus 危险色 |

必填字段必须同时具备：

- 视觉上的必填星号。
- `rules` 中的 `required: true` 校验。
- 保存前调用 `formRef.validate()`。
- 校验失败时停止接口请求，并定位到第一个错误字段。

只显示红色星号但仍可保存，视为功能缺陷。

### 8.3 输入控件

| 属性 | 标准值 |
| --- | --- |
| 默认高度 | `34px` |
| 圆角 | `8px`，筛选栏可用 `7px` |
| 背景 | `#fbfdff` 或白色 |
| 默认边框 | `#d9e5f2` |
| Hover 边框 | `#b9d4ff` |
| Focus 边框 | `#409eff` |
| 输入字号 | `12px - 13px` |
| Placeholder | `#94a3b8` |

```css
.ops-form :deep(.el-input__wrapper),
.ops-form :deep(.el-select__wrapper),
.ops-form :deep(.el-textarea__inner) {
  border-radius: 8px;
  background: #fbfdff;
  box-shadow: 0 0 0 1px #d9e5f2 inset;
}

.ops-form :deep(.el-input__wrapper:hover),
.ops-form :deep(.el-select__wrapper:hover) {
  box-shadow: 0 0 0 1px #b9d4ff inset;
}

.ops-form :deep(.el-input__wrapper.is-focus),
.ops-form :deep(.el-select__wrapper.is-focused) {
  box-shadow: 0 0 0 1px #409eff inset;
}
```

### 8.4 文本域和代码编辑器

- 普通说明文本域最小高度 `72px`。
- PromQL、日志查询、Shell、YAML 等技术内容使用等宽字体。
- 查询编辑器推荐行高 `1.65 - 1.7`，字号 `12px`。
- 脚本编辑器可使用深色背景，但必须作为明确的工具区域，不扩散到整个页面。
- 技术字段必须保留复制、格式化或全屏编辑能力，具体取决于业务需求。

### 8.5 单选、复选和开关

- 单选用于互斥选项，复选用于多选，开关用于即时启用或禁用。
- 不要用普通按钮模拟二元设置。
- 同组单选间距建议 `22px`，标签左间距 `7px`。
- 开关旁必须显示当前含义，避免只用颜色表达开启或关闭。

### 8.6 表单错误、禁用和加载

| 状态 | 表现 |
| --- | --- |
| 错误 | 红色边框、错误文字、必要时带错误图标 |
| 禁用 | 浅灰背景、文字减弱、鼠标为 `not-allowed` |
| 只读 | 保持可读对比度，与禁用态区分 |
| 提交中 | 保存按钮 Loading，表单避免重复提交 |
| 接口错误 | 弹出明确错误信息，保留用户输入 |

## 9. 表格规范

### 9.1 视觉

| 属性 | 标准值 |
| --- | --- |
| 外框圆角 | `8px - 10px` |
| 外框边框 | `#e3ebf5` |
| 表头背景 | `#f3f7fc` 或 `#f5f8fc` |
| 表头文字 | `12px / 700` |
| 单元格文字 | `12px` |
| 紧凑行高 | `34px - 40px` |
| 行分隔线 | `#eef3f9` |
| Hover 背景 | `#f8fafc` |

### 9.2 对齐

- 名称、描述、IP 等文本左对齐。
- 数量、百分比、金额右对齐。
- 状态标签居中或左对齐，但同一张表必须一致。
- 操作列右对齐，固定在最右侧。
- 长文本单行省略，并通过 Tooltip 展示完整值。

### 9.3 行内操作

- 高频操作最多展示 2 个文字或图标按钮。
- 其余操作收进更多菜单。
- 删除等危险操作必须在视觉和确认流程上与普通操作区分。
- 图标按钮必须提供 Tooltip 和可访问名称。

## 10. 图表规范

### 10.1 图表容器

- 图表必须放在稳定尺寸容器中，不能依赖内容自然撑高。
- 紧凑图表高度 `160px`；普通大盘图表建议 `172px - 210px`。
- 图表卡的标题、图例、绘图区要共享固定高度预算，避免绘图区过小。
- 无数据时显示同等高度的空状态，不折叠卡片。

### 10.2 颜色

- 同类图表使用稳定色序，不因数据排序改变业务含义。
- 严重程度固定为红、橙、蓝或黄，不与成功绿色混用。
- 饼图超过 6 - 8 类时合并为“其他”或改用条形图。
- 柱状图和折线图的网格线使用浅灰蓝，不使用纯黑。

### 10.3 标注

- 坐标轴字号 `11px`。
- 数值必须明确单位，如 `%`、核、GiB、台、次。
- Tooltip 中提供完整名称、数值、占比和时间。
- Top 5 / Top 10 必须在标题或筛选条件中表达清楚。

## 11. 状态、标签和提示

| 状态 | 主色 | 浅背景 | 示例 |
| --- | --- | --- | --- |
| 成功 / 正常 | `#059669` | `#eaf8ef` | 在线、恢复、执行成功 |
| 信息 / 处理中 | `#2563eb` | `#eef6ff` | 运行中、数据同步 |
| 警告 | `#d97706` | `#fff4e5` | 风险、即将到期 |
| 严重 / 失败 | `#dc2626` | `#fff0f0` | P0、离线、执行失败 |
| 中性 | `#64748b` | `#f3f7fc` | 未配置、未知 |

- 状态点建议 `7px - 8px`。
- 标签字号 `11px - 12px`，高度 `20px - 24px`。
- 标签可以使用 `999px` 圆角，但不要把整行列表做成胶囊形。
- 告警严重程度必须同时显示文字，不允许只显示红点或颜色。

## 12. 加载、空状态和错误状态

### 12.1 加载

- 首次加载使用卡片级 Skeleton 或 `v-loading`。
- 刷新时尽量保留旧数据，避免整个页面闪白。
- 按钮触发的异步操作在按钮内显示 Loading。
- 图表 Loading 和空状态必须保持容器原高度。

### 12.2 空状态

- 空状态说明发生了什么，例如“暂无活跃告警”。
- 有明确补救操作时提供一个主操作，例如“创建规则”。
- 不在每个空卡片中重复大尺寸插图，避免破坏大盘密度。

### 12.3 错误状态

- 页面局部接口失败时，仅让对应卡片进入错误态，不拖垮其他模块。
- 错误信息应包含业务对象和失败动作，例如“告警趋势加载失败”。
- 提供重试入口；开发错误栈不直接暴露给终端用户。

## 13. 图标规范

- 优先使用 `@element-plus/icons-vue`，保持项目图标语言一致。
- 工具操作优先图标按钮：刷新、关闭、折叠、复制、删除。
- 业务入口可使用“图标 + 文字”。
- 默认图标尺寸 `16px`；卡片标题图标 `14px - 17px`。
- 图标容器尺寸应固定，图标加载与否不能造成布局位移。
- 不手绘已有图标库可以提供的 SVG。

## 14. 响应式与可访问性

### 14.1 响应式

- 桌面优先设计，但 `960px` 以下必须可完成核心操作。
- 表单由多列自然降为单列，不缩小字号。
- 弹框在移动端减小外边距，而不是按比例缩放整个弹框。
- 表格列过多时优先隐藏低优先级列，必要时再启用横向滚动。
- 标题、按钮文字和最长单词不能溢出容器。

### 14.2 键盘和焦点

- 所有按钮、链接、可点击卡片都应支持键盘访问。
- 使用 `:focus-visible` 显示聚焦态，不对键盘用户移除焦点提示。
- 弹框打开后焦点进入弹框，关闭后回到触发按钮。
- `Esc` 关闭弹框前应考虑是否存在未保存数据。

### 14.3 对比度

- 正文对比度至少 `4.5:1`。
- 大字号文字和 UI 边界至少 `3:1`。
- 禁用态仍需可辨认，但不能看起来像可操作状态。
- 不依赖颜色作为唯一信息来源。

### 14.4 减少动态效果

```css
@media (prefers-reduced-motion: reduce) {
  *,
  *::before,
  *::after {
    scroll-behavior: auto !important;
    transition-duration: 0.01ms !important;
    animation-duration: 0.01ms !important;
    animation-iteration-count: 1 !important;
  }
}
```

## 15. Vue 与 Element Plus 实现约定

### 15.1 样式作用域

- 页面业务样式默认使用 `<style scoped lang="scss">` 或 `<style scoped>`。
- 修改当前组件内的 Element Plus 子节点使用 `:deep()`。
- 修改 Teleport 到 `body` 的 Dialog、Popover 等使用 `:global()` 配合专用 `modal-class` 或 `popper-class`。
- 不使用过宽的全局选择器覆盖所有 Element Plus 组件。

### 15.2 类名

推荐使用“模块 + 元素 + 状态”命名：

```text
.alert-ops-card
.alert-ops-card-title
.alert-ops-card.is-danger
.asset-summary-card--link
```

状态类使用 `is-*`，变体使用 `--*`。避免 `.box1`、`.left2`、`.new-style` 等无法表达含义的名称。

### 15.3 推荐组件骨架

```vue
<section class="ops-card">
  <header class="ops-card__header">
    <div>
      <h2>分区标题</h2>
      <p>只保留必要的上下文</p>
    </div>
    <el-button type="primary" :icon="Plus">创建</el-button>
  </header>

  <div class="ops-card__body">
    <!-- 图表、表格或表单 -->
  </div>
</section>
```

### 15.4 弹框骨架

```vue
<el-dialog
  v-model="dialogVisible"
  class="ops-dialog"
  modal-class="ops-form-dialog-overlay"
  width="760px"
  destroy-on-close
>
  <template #header>
    <div class="dialog-title">
      <span class="dialog-icon">
        <el-icon><Setting /></el-icon>
      </span>
      <div>
        <h2>创建监控规则</h2>
        <p>副标题仅补充当前对象或影响范围</p>
      </div>
    </div>
  </template>

  <el-form ref="formRef" :model="form" :rules="rules" label-position="top" class="ops-form">
    <div class="form-grid two-col">
      <!-- fields -->
    </div>
  </el-form>

  <template #footer>
    <div class="dialog-footer">
      <el-button @click="dialogVisible = false">取消</el-button>
      <el-button type="primary" :loading="submitting" @click="submit">保存</el-button>
    </div>
  </template>
</el-dialog>
```

## 16. 禁止事项

- 禁止用超大标题、营销式 Hero 或大面积留白处理运维工作台。
- 禁止无意义的渐变球、光斑、玻璃拟态和持续动画。
- 禁止每个区块都叠加重阴影。
- 禁止普通按钮、菜单项全部胶囊化。
- 禁止随意增加新的蓝色或紫色变体，状态色必须有稳定语义。
- 禁止在同一页面混用多个卡片圆角体系。
- 禁止仅修改弹框内部内容圆角而不修改 `.el-dialog` 外框。
- 禁止只显示必填星号却不阻止无效保存。
- 禁止让 Loading、空状态或错误状态改变卡片尺寸。
- 禁止用颜色作为告警级别、成功失败或开关状态的唯一提示。
- 禁止为熟悉的工具操作手绘 SVG，应使用现有图标库。

## 17. 页面验收清单

### 17.1 布局

- [ ] 页面内边距、模块间距符合 `7px / 8px` 的工作台节奏。
- [ ] 首屏没有大块无业务内容的空白。
- [ ] Grid 子项设置 `min-width: 0`，长文本不撑破布局。
- [ ] 桌面、中屏、小屏的列数按规范下降。
- [ ] 页面不存在不必要的双滚动条。

### 17.2 视觉

- [ ] 主卡片圆角为 `12px`，内部卡片为 `8px - 10px`。
- [ ] 普通弹框圆角为 `15px`，AI 助手例外为 `20px`。
- [ ] 阴影轻且统一，没有黑色重阴影。
- [ ] 标题、正文、辅助文字和数据值层级清晰。
- [ ] 渐变只用于主操作或 AI 强调区域。

### 17.3 交互

- [ ] 可点击元素具备 Hover、Active、Focus-visible、Disabled 状态。
- [ ] 图标按钮带 Tooltip。
- [ ] 提交按钮具备 Loading，避免重复提交。
- [ ] 必填字段校验失败时无法发起保存请求。
- [ ] 弹框焦点、关闭和未保存状态行为正确。

### 17.4 数据状态

- [ ] Loading、空状态、错误状态均已实现。
- [ ] 局部接口失败不会影响整个页面。
- [ ] 数值、时间、百分比和容量单位明确。
- [ ] 长文本提供省略与完整值查看方式。
- [ ] 图表容器在无数据时仍保持稳定高度。

## 18. 参考页面对应关系

| 规范内容 | 主要参考 |
| --- | --- |
| 紧凑页头、指标卡、资产分布、告警态势 | `Dashboard.vue` |
| 页面 `7px` 内边距和 `8px` 主间距 | `Dashboard.vue`、`systemd-dashboard.vue` |
| 主卡片 `12px`、弱边框和浅蓝阴影 | 两个参考页面 |
| 表单 `34px` 控件、`8px` 圆角、两到三列栅格 | `systemd-dashboard.vue` |
| 普通弹框 `15px` 圆角及 Teleport 作用域处理 | `systemd-dashboard.vue` |
| 暖到冷渐变、AI 对话和推理层次 | `systemd-dashboard.vue` |
| 紧凑表格、筛选栏和任务工作台 | `systemd-dashboard.vue` |

本规范中的数值以这两个页面当前已确认的视觉效果为基准。后续如果需要整体换肤，应先修改设计令牌，再由组件统一消费，避免逐页面覆盖。
