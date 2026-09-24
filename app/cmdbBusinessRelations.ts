// ==========================================================================
// AutoOps CMDB - 跨业务系统关联关系库 (Cross-System Business Relations)
// 依据人社、退军、工会、税务、北控等核心业务场景梳理的全链路业务互通模型
// ==========================================================================

export interface BusinessSystemRelation {
  id: string;
  sourceSystem: string;            // 本端系统名称 (匹配 EXCEL_PROJECTS)
  targetSystem: string;            // 关联系统名称
  relationType: "upstream" | "downstream" | "peer" | "shared_hub";
  relationTypeLabel: string;       // "上游业务入口" | "下游监管归集" | "平行协同联动" | "数据中台汇聚"
  direction: "incoming" | "outgoing" | "bidirectional"; // 数据/调用流向
  description: string;             // 业务协同场景与交互链路说明
  interfaceProtocol: string;       // 接口方式与通讯协议 (如 REST API, WebSocket, Kafka, ETL)
  frequency: "实时同步" | "准实时流" | "每日批处理" | "按需调用";
  dataEntities: string[];          // 核心交互数据项
  businessRole: string;            // 关联系统在当前链路中的角色
  status?: "normal" | "warning";   // 链路健康状态
}

/**
 * 核心预置业务系统关联关系矩阵
 * 深度覆盖仲裁集群、社保工伤集群、退军一体化、工会服务、税务征收、北控招采等真实业务
 */
export const PRESET_BUSINESS_RELATIONS: BusinessSystemRelation[] = [
  // ================= 1. 仲裁业务系统集群 (核心关注) =================
  {
    id: "rel-zc-1",
    sourceSystem: "仲裁核心系统",
    targetSystem: "仲裁网厅系统",
    relationType: "upstream",
    relationTypeLabel: "上游网上申报入口",
    direction: "incoming",
    description: "网厅作为互联网申报前端，承接企事业单位与劳动者的网上立案申请、答辩申请、文书送达回证，并将校验后的案件材料实时推送至仲裁核心系统进行立案审查。",
    interfaceProtocol: "RESTful API (HTTPS/JSON / 国密 SM2/SM4 签名加密)",
    frequency: "实时同步",
    dataEntities: ["网上立案申请单", "当事人诉辩材料", "电子证据包", "送达确认回执"],
    businessRole: "互联网公众与代理人申报前端门户",
    status: "normal"
  },
  {
    id: "rel-zc-2",
    sourceSystem: "仲裁核心系统",
    targetSystem: "仲裁云庭审系统",
    relationType: "peer",
    relationTypeLabel: "平行协同庭审系统",
    direction: "bidirectional",
    description: "核心系统完成排期排庭后，向云庭审系统下发开庭指令与参会码；云庭审系统在开庭过程中支撑多方音视频连线，庭审结束后将电子庭审笔录、签字回执及庭审录像推回核心系统归卷备查。",
    interfaceProtocol: "WebSocket 实时流 + REST API + WebRTC 音视频传输",
    frequency: "实时同步",
    dataEntities: ["庭审排期计划", "当事人入庭鉴权", "数字化语音笔录", "庭审归档音视频"],
    businessRole: "在线庭审排期、云视频连线与笔录签署平台",
    status: "normal"
  },
  {
    id: "rel-zc-3",
    sourceSystem: "仲裁核心系统",
    targetSystem: "调解仲裁系统",
    relationType: "peer",
    relationTypeLabel: "案前调解诉调对接",
    direction: "bidirectional",
    description: "支撑基层人社服务所及企事业单位开展案前多元调解；调解成功的出具调解协议书，调解不成的案件一键转入仲裁核心系统进入正式立案程序，实现案前案中一体化流转。",
    interfaceProtocol: "REST API + MQ 异步案件状态队列",
    frequency: "实时同步",
    dataEntities: ["案前调解申请", "调解登记台账", "调解结案/转立案凭据", "当事人基本身份核验"],
    businessRole: "基层多元矛盾化解与案前调解业务支撑",
    status: "normal"
  },
  {
    id: "rel-zc-4",
    sourceSystem: "仲裁核心系统",
    targetSystem: "监管中台/工伤红名单/即席查询",
    relationType: "downstream",
    relationTypeLabel: "下游风控与监管中台",
    direction: "outgoing",
    description: "仲裁核心系统实时将劳动纠纷涉案单位、失信被执行企业、恶意拖欠薪酬案件向监管中台同步，作为全市企业用工风险评估与红黑名单预警的重要决策依据。",
    interfaceProtocol: "Kafka 分布式消息通道 + REST 数据推送",
    frequency: "准实时流",
    dataEntities: ["仲裁案件结案结论", "涉案用工单位信用", "劳动报酬纠纷案件清单"],
    businessRole: "全市人社综合风控预警与多维监管即席查询中枢",
    status: "normal"
  },
  {
    id: "rel-zc-5",
    sourceSystem: "仲裁核心系统",
    targetSystem: "数转区",
    relationType: "shared_hub",
    relationTypeLabel: "局级大数据湖与数转区",
    direction: "outgoing",
    description: "每日定时将全量增量仲裁业务数据清洗脱敏后导入数转区，支撑全市政府大数据平台共享交换、综合运行态势大屏及历史案件档案归集。",
    interfaceProtocol: "ETL 数据抽取流水线 / 共享库 DataBridge",
    frequency: "每日批处理",
    dataEntities: ["全量案件流转日志", "统计分析指标宽表", "文书归档索引"],
    businessRole: "政务数据要素流通、大数据清洗与共享交换枢纽",
    status: "normal"
  },

  // 仲裁网厅系统为源的反向/关联关系
  {
    id: "rel-zc-6",
    sourceSystem: "仲裁网厅系统",
    targetSystem: "仲裁核心系统",
    relationType: "downstream",
    relationTypeLabel: "核心业务处理中枢",
    direction: "outgoing",
    description: "网厅将收取的网上申报、当事人证据补充推送至内网核心仲裁系统，并接收核心系统办理进度反馈至网厅供市民查询。",
    interfaceProtocol: "RESTful API (DMZ 外网穿透到政务外网通道)",
    frequency: "实时同步",
    dataEntities: ["申报立案信息", "证据材料", "办理进度状态更新"],
    businessRole: "劳动人事争议裁决处理与案件流转核心中枢",
    status: "normal"
  },
  {
    id: "rel-zc-7",
    sourceSystem: "仲裁网厅系统",
    targetSystem: "调解仲裁系统",
    relationType: "peer",
    relationTypeLabel: "网厅案前调解引导",
    direction: "bidirectional",
    description: "在网厅前端为市民提供案前调解智能引导服务，支持在线提交调解意愿，分流轻微劳动争议纠纷至基层调解平台。",
    interfaceProtocol: "REST API 服务调用",
    frequency: "按需调用",
    dataEntities: ["调解诉求工单", "调解员指派信息", "当事人调解意向书"],
    businessRole: "前端案前矛盾分流与线上调解通道",
    status: "normal"
  },

  // 仲裁云庭审系统为源的关系
  {
    id: "rel-zc-8",
    sourceSystem: "仲裁云庭审系统",
    targetSystem: "仲裁核心系统",
    relationType: "upstream",
    relationTypeLabel: "案源排期与归档主干",
    direction: "bidirectional",
    description: "接收核心仲裁系统排庭排期指令，反馈庭审到场签到状态，并输出完整电子笔录与庭审实况数据回传至案卷中心。",
    interfaceProtocol: "WebSocket + REST API 专线传输",
    frequency: "实时同步",
    dataEntities: ["排庭日程", "庭审电子签名", "录音录像媒体包"],
    businessRole: "案件承办人审查审理与裁判裁决业务中台",
    status: "normal"
  },
  {
    id: "rel-zc-9",
    sourceSystem: "仲裁云庭审系统",
    targetSystem: "仲裁网厅系统",
    relationType: "peer",
    relationTypeLabel: "网厅当事人开庭入口",
    direction: "bidirectional",
    description: "网厅作为当事人与代理人参加入庭的统一身份认证入口，通过免密单点登录 (SSO) 安全跳转入云庭审虚拟庭审室。",
    interfaceProtocol: "OAuth2.0 / SAML 统一单点登录认证 + Token 鉴权",
    frequency: "按需调用",
    dataEntities: ["当事人身份校验 Token", "云庭审房间直达链接"],
    businessRole: "外网市民与法务代理人开庭访问端",
    status: "normal"
  },

  // 调解仲裁系统为源的关系
  {
    id: "rel-zc-10",
    sourceSystem: "调解仲裁系统",
    targetSystem: "仲裁核心系统",
    relationType: "downstream",
    relationTypeLabel: "立案中枢与司法裁决",
    direction: "outgoing",
    description: "对于经多轮调解未能达成一致的劳资争议，由调解系统自动打包案件事由和调解纪要，向仲裁核心系统发起转立案审查。",
    interfaceProtocol: "REST API 流程对接",
    frequency: "实时同步",
    dataEntities: ["争议事由概要", "前期调解笔录", "立案申请资料"],
    businessRole: "正式劳动仲裁裁决与终审业务系统",
    status: "normal"
  },

  // ================= 2. 社保与工伤业务系统集群 =================
  {
    id: "rel-sb-1",
    sourceSystem: "原三险系统",
    targetSystem: "原工伤认定系统",
    relationType: "peer",
    relationTypeLabel: "工伤参保资格与待遇对账",
    direction: "bidirectional",
    description: "在进行工伤认定时，工伤系统实时调用三险系统接口核验事故发生时用人单位工伤保险参保缴费状态；工伤认定后将工伤评级与待遇报销信息返回三险系统进行基金支出拨付。",
    interfaceProtocol: "REST API 微服务调用 / 高速专线 RPC",
    frequency: "实时同步",
    dataEntities: ["职工参保缴费基数", "险种参保起止期", "工伤认定结论书", "工伤伤残等级评定"],
    businessRole: "工伤事故申报、调查核实与行政认定系统",
    status: "normal"
  },
  {
    id: "rel-sb-2",
    sourceSystem: "原三险系统",
    targetSystem: "原城居系统",
    relationType: "peer",
    relationTypeLabel: "城乡养老跨制度转移衔接",
    direction: "bidirectional",
    description: "支撑城镇职工养老保险与城乡居民基本养老保险制度之间的参保人员权益互转、个人账户累计储存额与缴费年限折算对账。",
    interfaceProtocol: "数据库链路 (DB Link) + 业务中台消息机制",
    frequency: "每日批处理",
    dataEntities: ["个人账户积累额", "跨险种转移凭证", "重复参保退费清算表"],
    businessRole: "城乡居民基本养老保险征缴与待遇发放中枢",
    status: "normal"
  },
  {
    id: "rel-sb-3",
    sourceSystem: "原三险系统",
    targetSystem: "监管中台/工伤红名单/即席查询",
    relationType: "downstream",
    relationTypeLabel: "社保反欺诈与红黑名单风控",
    direction: "outgoing",
    description: "将三险参保异常、冒领社保待遇疑点、重复参保等交易明细同步至监管中台，配合即席查询开展全险种资金流向与社保基金反欺诈筛查。",
    interfaceProtocol: "Kafka 消息中间件 + 准实时流处理引擎",
    frequency: "准实时流",
    dataEntities: ["待遇发放支付流水", "参保人员生存认证状态", "用人单位欠费预警"],
    businessRole: "全市社保风控态势感知与多维监管分析平台",
    status: "normal"
  },
  {
    id: "rel-sb-4",
    sourceSystem: "原三险系统",
    targetSystem: "税务年金征收系统",
    relationType: "peer",
    relationTypeLabel: "税务代征征缴对账通道",
    direction: "bidirectional",
    description: "实现社保经办与税务全责征收的专网专线互联，每日比对企业参保申报应征数与税务实际划缴入库数，确保社保基金与年金账实相符。",
    interfaceProtocol: "政务专网安全前置机 + 国密专线 SFTP / XML 报文",
    frequency: "每日批处理",
    dataEntities: ["社保核定应征台账", "税务入库对账流水", "差额退补清单"],
    businessRole: "税务部门企业年金与社保非税征收管理系统",
    status: "normal"
  },
  {
    id: "rel-sb-5",
    sourceSystem: "原三险系统",
    targetSystem: "数转区",
    relationType: "shared_hub",
    relationTypeLabel: "人社大数据湖全量归集",
    direction: "outgoing",
    description: "历史三险台账全量归集入数转区，沉淀数十亿条参保人历史权益记录，支撑人社部统筹系统迁移与全市便民数智化分析。",
    interfaceProtocol: "Hadoop/Hive/ODPS 大数据批量同步工具",
    frequency: "每日批处理",
    dataEntities: ["参保历史全量台账", "历年缴费流水底表", "待遇计发历史明细"],
    businessRole: "人社数据要素汇聚与数字化转型底座",
    status: "normal"
  },

  // 原工伤认定系统为源的关系
  {
    id: "rel-gs-1",
    sourceSystem: "原工伤认定系统",
    targetSystem: "原三险系统",
    relationType: "peer",
    relationTypeLabel: "工伤参保状态校验与待遇发放",
    direction: "bidirectional",
    description: "受理工伤认定案件时实时检索申请人三险工伤参保状态，认定结案后推送结果至三险系统触发工伤医疗费用报销和伤残津贴计发。",
    interfaceProtocol: "RESTful API / RPC 专线调用",
    frequency: "实时同步",
    dataEntities: ["职工参保状态校验结果", "工伤认定决定书编号", "工伤医疗基金垫付核销"],
    businessRole: "基本工伤保险基金经办与待遇计发中枢",
    status: "normal"
  },
  {
    id: "rel-gs-2",
    sourceSystem: "原工伤认定系统",
    targetSystem: "监管中台/工伤红名单/即席查询",
    relationType: "downstream",
    relationTypeLabel: "工伤红名单与事故风控比对",
    direction: "outgoing",
    description: "高频发生工伤事故单位、安全生产红黑名单企业自动推送到监管中台，联动多维即席查询进行行业安全生产风险预警。",
    interfaceProtocol: "API 实时事件推送",
    frequency: "实时同步",
    dataEntities: ["事故频发单位名录", "涉案企业信用标记", "职业病诊断备案清单"],
    businessRole: "工伤领域红黑名单库与行政监管查询中枢",
    status: "normal"
  },
  {
    id: "rel-gs-3",
    sourceSystem: "原工伤认定系统",
    targetSystem: "数转区",
    relationType: "shared_hub",
    relationTypeLabel: "工伤数据标准化汇聚",
    direction: "outgoing",
    description: "定期将认定办件、复议诉讼及医疗核实数据归集至数转区，支撑全市工伤预防模型训练与宏观态势分析。",
    interfaceProtocol: "DataX / ETL 批量抽取",
    frequency: "每日批处理",
    dataEntities: ["工伤认定全生命周期表", "工伤事故类型统计", "伤残等级分布分布表"],
    businessRole: "人社数据资产池与共享服务中枢",
    status: "normal"
  },

  // 监管中台为源的关系
  {
    id: "rel-jg-1",
    sourceSystem: "监管中台/工伤红名单/即席查询",
    targetSystem: "原三险系统",
    relationType: "upstream",
    relationTypeLabel: "社保待遇反欺诈风控预警",
    direction: "incoming",
    description: "从三险系统抽取全量支付与核算日志，进行实时欺诈模型测算，发现异常时向三险系统发出待遇拦截与人工复审预警。",
    interfaceProtocol: "Kafka 消费 + 预警回调 API",
    frequency: "准实时流",
    dataEntities: ["待遇发放核验指令", "高风险阻断预警清单", "多重身份参保比对报告"],
    businessRole: "社保基本盘业务与养老工伤失业经办核心",
    status: "normal"
  },
  {
    id: "rel-jg-2",
    sourceSystem: "监管中台/工伤红名单/即席查询",
    targetSystem: "仲裁核心系统",
    relationType: "upstream",
    relationTypeLabel: "劳动人事争议裁决涉案企业信息源",
    direction: "incoming",
    description: "接入仲裁裁判裁决数据，将恶意拖欠农民工薪资、拒不履行仲裁调解协议的企业列入劳动用工重点监管红黑名单。",
    interfaceProtocol: "REST API 数据同步",
    frequency: "准实时流",
    dataEntities: ["重大仲裁涉案企业清单", "拖欠薪资黑名单", "仲裁裁决履约跟踪记录"],
    businessRole: "劳动仲裁案件裁处核心系统",
    status: "normal"
  },
  {
    id: "rel-jg-3",
    sourceSystem: "监管中台/工伤红名单/即席查询",
    targetSystem: "数转区",
    relationType: "peer",
    relationTypeLabel: "大数据融合与即席查询加速",
    direction: "bidirectional",
    description: "依托数转区底层分布式计算与列存索引引擎，为各级监管执法人员提供数十亿级人社资产台账的秒级即席多维穿透查询服务。",
    interfaceProtocol: "ClickHouse / StarRocks / Elasticsearch 跨源查询联邦",
    frequency: "按需调用",
    dataEntities: ["即席查询执行结果集", "监管预警大屏指标", "跨业务关联分析报表"],
    businessRole: "人社大数据高性能分析计算与交换底座",
    status: "normal"
  },

  // 数转区为源的关系
  {
    id: "rel-sz-1",
    sourceSystem: "数转区",
    targetSystem: "原三险系统",
    relationType: "peer",
    relationTypeLabel: "历史台账沉淀与数仓回流",
    direction: "incoming",
    description: "承担原三险系统数十载历史海量档案数据的数字化转型底座，提供安全沙箱隔离与历史台账归档查询服务。",
    interfaceProtocol: "ETL 数据管线",
    frequency: "每日批处理",
    dataEntities: ["历史结转参保台账", "历史个人账户明细", "档案影像索引"],
    businessRole: "核心生产业务系统",
    status: "normal"
  },
  {
    id: "rel-sz-2",
    sourceSystem: "数转区",
    targetSystem: "仲裁核心系统",
    relationType: "peer",
    relationTypeLabel: "仲裁数据资产化清洗",
    direction: "incoming",
    description: "汇聚仲裁裁决知识库、相似案例法律条款匹配库，经过自然语言与要素清洗后赋能仲裁核心系统辅助裁判建议。",
    interfaceProtocol: "REST 知识库服务",
    frequency: "按需调用",
    dataEntities: ["类案智能检索结果", "裁判文书模板库", "人社法律法规标准库"],
    businessRole: "劳动人事仲裁办案办席中枢",
    status: "normal"
  },

  // ================= 3. 退役军人业务系统集群 =================
  {
    id: "rel-tj-1",
    sourceSystem: "退军补贴发放系统",
    targetSystem: "退军资金一体化监管-企转系统",
    relationType: "downstream",
    relationTypeLabel: "发放资金监管与企转核对",
    direction: "bidirectional",
    description: "补贴发放系统生成退役军人生活补助、军转干部安置费拨付清单后，自动提交至一体化监管企转系统审核资金专户余额，完成资金流转全生命周期合规监管与银行回单对账。",
    interfaceProtocol: "国密通道 REST API + 银行直联前置机",
    frequency: "实时同步",
    dataEntities: ["退军补贴发放批次", "优抚对象资金凭证", "银行代发成功回执", "企转安置专项资金台账"],
    businessRole: "退役军人专项资金全流程穿透式监管平台",
    status: "normal"
  },
  {
    id: "rel-tj-2",
    sourceSystem: "退军资金一体化监管-企转系统",
    targetSystem: "退军补贴发放系统",
    relationType: "upstream",
    relationTypeLabel: "资金审批流转与额度管控",
    direction: "bidirectional",
    description: "实时监控财政专户划拨额度与企业转业安置补贴分配额度，审核通过后回传发放令牌，保障每笔退役军人补贴精准安全发放到人。",
    interfaceProtocol: "REST API + 电子签章鉴权",
    frequency: "实时同步",
    dataEntities: ["财政拨付审批文书", "资金发放确认令牌", "异常冻结拦截指令"],
    businessRole: "优抚对象定期抚恤与补贴直达发放系统",
    status: "normal"
  },

  // ================= 4. 总工会保障业务系统集群 =================
  {
    id: "rel-gh-1",
    sourceSystem: "工会互助保险信息系统",
    targetSystem: "工会职服数智化系统",
    relationType: "peer",
    relationTypeLabel: "会员权益与二次报销理赔联动",
    direction: "bidirectional",
    description: "工会职服数智化系统作为职工手机端与企事业工会前置入口，向职工提供暖心服务；当职工在线发起互助保障理赔申请时，自动将社保报销单据同步给工会互助保险信息系统进行二次赔付审核。",
    interfaceProtocol: "RESTful API (HTTPS) + 工会卡统一身份认证体系",
    frequency: "实时同步",
    dataEntities: ["工会会员身份鉴权", "在职职工互助保险投保单", "医疗互助二次报销理赔凭单"],
    businessRole: "北京市总工会职工普惠数智化服务与会员移动门户",
    status: "normal"
  },
  {
    id: "rel-gh-2",
    sourceSystem: "工会职服数智化系统",
    targetSystem: "工会互助保险信息系统",
    relationType: "peer",
    relationTypeLabel: "互助保障投保与理赔核销中枢",
    direction: "bidirectional",
    description: "为职工提供一站式保单查询、理赔进度追踪与暖心互助慰问金直发，与互助保险后台结算数据库实现秒级无缝联动。",
    interfaceProtocol: "REST API + WebSocket 进度消息推送",
    frequency: "实时同步",
    dataEntities: ["职工参保状态凭证", "理赔金拨付到卡结果", "工会关爱服务卡权益包"],
    businessRole: "职工互助保障政策、保单精算与资金结算管理平台",
    status: "normal"
  },

  // ================= 5. 税务年金征收系统 =================
  {
    id: "rel-sw-1",
    sourceSystem: "税务年金征收系统",
    targetSystem: "原三险系统",
    relationType: "upstream",
    relationTypeLabel: "社保与企业年金税务代征对账通道",
    direction: "bidirectional",
    description: "由税务部门全面代征企业年金与机关事业单位养老缴费，专线每日对齐应征计划与入库完税证明，保障非税征管信息闭环。",
    interfaceProtocol: "金税三期/四期政务专线 + 安全交换前置机",
    frequency: "每日批处理",
    dataEntities: ["企业年金缴费明细", "税费完税电子凭据", "多缴重缴退库对账单"],
    businessRole: "社保经办核心结算中枢",
    status: "normal"
  },

  // ================= 6. 北控集团招采与数字化平台集群 =================
  {
    id: "rel-bk-1",
    sourceSystem: "北控集团招采平台",
    targetSystem: "北控集团商城平台",
    relationType: "downstream",
    relationTypeLabel: "集采物资与框架协议自动上架",
    direction: "outgoing",
    description: "招采平台完成年度集采招标与框架协议签订后，将中标供应商、入围商品规格、协议采购价自动推送至北控商城平台上架，实现阳光采购与按需下单闭环。",
    interfaceProtocol: "REST API + Webhook 采购订单通知",
    frequency: "实时同步",
    dataEntities: ["框架协议中标物料库", "入围合格供应商名录", "集采限价标准清单"],
    businessRole: "集团企业内部员工采购与协同物资商城",
    status: "normal"
  },
  {
    id: "rel-bk-2",
    sourceSystem: "北控集团招采平台",
    targetSystem: "北控环境招采系统",
    relationType: "peer",
    relationTypeLabel: "集团专业板块招采协同",
    direction: "bidirectional",
    description: "共享集团专家评审专家库、合格供应商黑白名单及统一电子招投标合规审计规范，支持下属环境专业板块招投标数据一键上报集团监管。",
    interfaceProtocol: "REST API 共享服务",
    frequency: "准实时流",
    dataEntities: ["评审专家抽签结果", "供应商信用共享档案", "工程与环保装备招标方案"],
    businessRole: "环保能源板块专业工程招投标与采购平台",
    status: "normal"
  },
  {
    id: "rel-bk-3",
    sourceSystem: "北控集团招采平台",
    targetSystem: "数科精益管理平台",
    relationType: "peer",
    relationTypeLabel: "招采预算与成本支出闭环",
    direction: "bidirectional",
    description: "采购立项前校验数科精益平台中的项目财务预算额度；采购决标后将实际中标金额回写精益平台冻结并列支对应预算科目。",
    interfaceProtocol: "财务 ERP 接口 / REST API",
    frequency: "实时同步",
    dataEntities: ["采购项目预算申请单", "中标决标金额核销凭证", "供应商合同支付节点"],
    businessRole: "北控数科精益运营与预算成本管控系统",
    status: "normal"
  },
  {
    id: "rel-bk-4",
    sourceSystem: "北控集团商城平台",
    targetSystem: "北控集团招采平台",
    relationType: "upstream",
    relationTypeLabel: "商城采购订单与协议履约监管",
    direction: "incoming",
    description: "商城平台汇总各分子公司对协议供货商品的采购订单流水，定期向招采平台同步履约执行进度，作为供应商考核的重要评定参数。",
    interfaceProtocol: "REST API",
    frequency: "每日批处理",
    dataEntities: ["采购下单明细流水", "供应商发货履约时效", "商品退换货及评价汇总"],
    businessRole: "集团大宗物资采购与电子化招标管理中枢",
    status: "normal"
  },
  {
    id: "rel-bk-5",
    sourceSystem: "燃气内管/高山监管平台/北控资产管理",
    targetSystem: "伟仕精益管理平台",
    relationType: "peer",
    relationTypeLabel: "管网设备资产运维与精益工单联动",
    direction: "bidirectional",
    description: "高山监控平台与燃气内管检测发现管网设备异常或巡检预警时，自动在伟仕精益管理平台生成抢修抢险与日常维护运维工单，实现资产与运维闭环。",
    interfaceProtocol: "SCADA / 物联网网关 MQTT + REST API 工单触发",
    frequency: "实时同步",
    dataEntities: ["管网巡检告警事件", "资产设备位置台账", "现场抢修工单执行回执"],
    businessRole: "燃气工程现场精益化运营与运维调度平台",
    status: "normal"
  }
];

export const BUSINESS_RELATIONS_STORAGE_KEY = "autoops_business_relations_v1";

/**
 * 智能检索指定业务系统的所有关联系统（含预置业务关系与同单位派生协同）
 */
export function getRelatedBusinessSystems(
  systemName: string,
  allSystemNames: string[] = [],
  customerName?: string,
  sourceRelations: BusinessSystemRelation[] = PRESET_BUSINESS_RELATIONS
): {
  upstream: BusinessSystemRelation[];
  peer: BusinessSystemRelation[];
  downstream: BusinessSystemRelation[];
  all: BusinessSystemRelation[];
} {
  const activeRelations = sourceRelations && sourceRelations.length > 0 ? sourceRelations : PRESET_BUSINESS_RELATIONS;
  // 1. 获取关联库中以该系统为 source 的所有关系
  const directRelations = activeRelations.filter(r => r.sourceSystem === systemName);

  // 2. 获取关联库中以该系统为 target 的反向关系（可转换为当前系统的相对关系）
  const incomingTargetRelations = activeRelations.filter(r => r.targetSystem === systemName);
  const derivedReverseRelations: BusinessSystemRelation[] = [];

  for (const inv of incomingTargetRelations) {
    // 检查是否在 directRelations 中已有对应的互斥记录
    const exists = directRelations.some(d => d.targetSystem === inv.sourceSystem);
    if (!exists) {
      // 逆转视角：如果对方是 upstream，从我的视角看它是 downstream，反之亦然
      let invertedType: BusinessSystemRelation["relationType"] = "peer";
      let invertedLabel = "业务互通系统";
      let invertedDirection: BusinessSystemRelation["direction"] = "bidirectional";

      if (inv.relationType === "upstream") {
        invertedType = "downstream";
        invertedLabel = "下游处理系统";
        invertedDirection = inv.direction === "incoming" ? "outgoing" : "bidirectional";
      } else if (inv.relationType === "downstream") {
        invertedType = "upstream";
        invertedLabel = "上游协同系统";
        invertedDirection = inv.direction === "outgoing" ? "incoming" : "bidirectional";
      } else if (inv.relationType === "shared_hub") {
        invertedType = "shared_hub";
        invertedLabel = "数据中台汇聚端";
      } else {
        invertedType = "peer";
        invertedLabel = "协同联动系统";
      }

      derivedReverseRelations.push({
        id: `rev-${inv.id}`,
        sourceSystem: systemName,
        targetSystem: inv.sourceSystem,
        relationType: invertedType,
        relationTypeLabel: invertedLabel,
        direction: invertedDirection,
        description: `与【${inv.sourceSystem}】具备业务协同互动链路：${inv.description}`,
        interfaceProtocol: inv.interfaceProtocol,
        frequency: inv.frequency,
        dataEntities: inv.dataEntities,
        businessRole: `${inv.sourceSystem}业务交互方`,
        status: inv.status
      });
    }
  }

  const combinedRelations = [...directRelations, ...derivedReverseRelations];

  // 3. 若同单位或同属于北控体系但尚无明确链路的项目，自动派生“同域业务协同网”
  if (combinedRelations.length === 0 && customerName) {
    const peersInSameCustomer = allSystemNames.filter(
      s => s !== systemName && (
        (customerName.includes("北控") && s.includes("北控")) ||
        (customerName.includes("人社") && (s.includes("系统") || s.includes("平台")))
      )
    );

    for (const p of peersInSameCustomer.slice(0, 3)) {
      combinedRelations.push({
        id: `auto-${p}`,
        sourceSystem: systemName,
        targetSystem: p,
        relationType: "peer",
        relationTypeLabel: "同集团/同单位协同业务",
        direction: "bidirectional",
        description: `同属于【${customerName}】数字化资产管理集群，共享统一身份认证、企业级安全接入网关与专有数据交互通道。`,
        interfaceProtocol: "统一 API 认证网关 / SSO 单点登录 / 国密传输",
        frequency: "按需调用",
        dataEntities: ["统一组织机构主数据", "单点登录认证 Token", "跨部门业务交互工单"],
        businessRole: `${customerName} 数字化兄弟系统`,
        status: "normal"
      });
    }
  }

  // 4. 分类归纳
  const upstream = combinedRelations.filter(r => r.relationType === "upstream");
  const downstream = combinedRelations.filter(r => r.relationType === "downstream" || r.relationType === "shared_hub");
  const peer = combinedRelations.filter(r => r.relationType === "peer");

  return {
    upstream,
    peer,
    downstream,
    all: combinedRelations
  };
}
