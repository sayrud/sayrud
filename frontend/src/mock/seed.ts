import dayjs from 'dayjs'

import type {
  CellValue,
  FieldMetadata,
  FieldType,
  Project,
  SelectOption,
  SLField,
  SLRecord,
  SLTable,
  SLView,
  ViewConfig,
  ViewType,
} from '@/types/bitable'
import { newFieldUID, newOptionUID, newProjectUID, newRecordUID, newTableUID, newViewUID } from '@/utils/id'
import { defaultViewConfig } from '@/utils/view'

// 固定种子的伪随机数，保证示例数据每次生成一致。
function mulberry32(seed: number) {
  return () => {
    seed |= 0
    seed = (seed + 0x6d2b79f5) | 0
    let t = Math.imul(seed ^ (seed >>> 15), 1 | seed)
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296
  }
}

interface FieldDef {
  label: string
  type: FieldType
  metadata?: Partial<FieldMetadata>
  /** 单选 / 多选的选项：[名称, 颜色下标]。 */
  options?: [string, number][]
  /** 公式中用 {字段名} 引用，生成时替换为 UID。 */
  exp?: string
}

interface ViewDef {
  name: string
  type: ViewType
  config?: (f: (label: string) => string) => Partial<ViewConfig>
}

interface TableDef {
  name: string
  fields: FieldDef[]
  rows: Record<string, unknown>[]
  views: ViewDef[]
}

interface Seed {
  projects: Project[]
  tables: SLTable[]
  fields: SLField[]
  records: SLRecord[]
  views: SLView[]
}

function buildTable(seed: Seed, projectUID: string, def: TableDef, createdAt: dayjs.Dayjs) {
  const table: SLTable = {
    uid: newTableUID(),
    projectUID,
    name: def.name,
    createdAt: createdAt.toISOString(),
    updatedAt: createdAt.toISOString(),
  }
  seed.tables.push(table)

  const fields: SLField[] = def.fields.map((fd, i) => {
    const base: Record<string, unknown> = { ...(fd.metadata ?? {}) }
    if (fd.type === 'single_select' || fd.type === 'multi_select') {
      base.options = (fd.options ?? []).map(([name, color]): SelectOption => ({ uid: newOptionUID(), name, color }))
      base.default ??= fd.type === 'multi_select' ? [] : ''
    }
    const defaults: Record<FieldType, Record<string, unknown>> = {
      text: { default: '' },
      number: { format: '0', default: null },
      datetime: { format: 'YYYY/MM/DD', with_time: false, default: '' },
      checkbox: {},
      formula: { exp: '' },
      single_select: {},
      multi_select: {},
    }
    return {
      uid: newFieldUID(),
      tableUID: table.uid,
      label: fd.label,
      type: fd.type,
      metadata: { ...defaults[fd.type], ...base } as SLField['metadata'],
      position: i,
      createdAt: table.createdAt,
      updatedAt: table.createdAt,
    }
  })
  const byLabel = new Map(fields.map((f) => [f.label, f]))
  def.fields.forEach((fd, i) => {
    if (fd.type === 'formula' && fd.exp) {
      const exp = fd.exp.replace(/\{([^}]+)\}/g, (_m, label: string) => `{${byLabel.get(label)!.uid}}`)
      ;(fields[i]!.metadata as { exp: string }).exp = exp
    }
  })
  seed.fields.push(...fields)

  def.rows.forEach((row, i) => {
    const data: Record<string, CellValue> = {}
    for (const [label, value] of Object.entries(row)) {
      const f = byLabel.get(label)
      if (!f || value === undefined || value === null || value === '') continue
      const opts = (f.metadata as { options?: SelectOption[] }).options ?? []
      if (f.type === 'single_select') data[f.uid] = opts.find((o) => o.name === value)?.uid ?? null
      else if (f.type === 'multi_select')
        data[f.uid] = (value as string[]).map((n) => opts.find((o) => o.name === n)?.uid).filter(Boolean) as string[]
      else if (f.type === 'datetime') data[f.uid] = dayjs(value as string).toISOString()
      else data[f.uid] = value as CellValue
    }
    const t = createdAt.add(i, 'minute').toISOString()
    seed.records.push({ uid: newRecordUID(), tableUID: table.uid, data, createdAt: t, updatedAt: t })
  })

  const f = (label: string) => byLabel.get(label)!.uid
  def.views.forEach((vd, i) => {
    seed.views.push({
      uid: newViewUID(),
      tableUID: table.uid,
      name: vd.name,
      type: vd.type,
      position: i,
      config: defaultViewConfig(vd.config?.(f) ?? {}),
    })
  })
}

export function buildSeed(): Seed {
  const rand = mulberry32(20260926)
  const pick = <T>(arr: T[]): T => arr[Math.floor(rand() * arr.length)]!
  const today = dayjs().startOf('day')
  const seed: Seed = { projects: [], tables: [], fields: [], records: [], views: [] }

  const mkProject = (name: string, schemaName: string, daysAgo: number): Project => {
    const t = today.subtract(daysAgo, 'day').hour(10).toISOString()
    const p = { uid: newProjectUID(), name, schemaName, createdAt: t, updatedAt: t }
    seed.projects.push(p)
    return p
  }

  // ---- 项目一：产品研发管理 ----
  const rd = mkProject('产品研发管理', 'product_rd', 30)
  const people = ['张三', '李四', '王五', '赵六', '孙七', '周八']
  const titles = [
    '支持按单选字段分组展示',
    '表格视图支持冻结列',
    '新增看板视图拖拽排序',
    '字段支持自定义默认值',
    '表单视图支持必填校验',
    '公式字段支持 DATEDIF 函数',
    '大数据量下虚拟滚动优化',
    '数据表支持复制',
    '记录详情支持上一条 / 下一条切换',
    '筛选条件支持"任一"逻辑',
    '导出 CSV',
    '批量粘贴自动新增选项',
    '列宽拖拽调整并记忆',
    '单元格支持撤销 / 重做',
    '开放 API 鉴权改造',
    '行高设置（低 / 中 / 高 / 超高）',
    '视图级别的字段隐藏',
    '统计栏支持求和与平均值',
    '日期字段支持时间精度',
    '数字字段支持货币格式',
    '搜索并高亮匹配单元格',
    '画册视图卡片布局',
    '权限：只读成员不可编辑',
    '移动端表格适配',
    '数据表回收站',
    '字段描述与帮助文本',
    '选项颜色自定义',
    '分组支持折叠与展开',
    '多级排序',
    'Webhook 推送记录变更',
    'AI 生成数据表结构',
    '新手引导与模板中心',
    '操作日志审计',
    '看板列按选项顺序排列',
    '表头右键菜单',
    '复选框批量勾选',
  ]
  const rdRows = titles.map((title, i) => {
    const status = pick(['待评估', '待开发', '开发中', '开发中', '测试中', '已上线', '已上线', '已搁置'])
    const start = today.add(Math.floor(rand() * 30) - 20, 'day')
    const end = start.add(3 + Math.floor(rand() * 20), 'day')
    const progress =
      status === '已上线' ? 1 : status === '测试中' ? 0.8 : status === '开发中' ? Math.round(rand() * 6 + 2) / 10 : 0
    const modules = ['多维表格', '表单', '权限', '开放平台', '性能', '体验']
    return {
      需求标题: title,
      状态: status,
      优先级: pick(['P0', 'P1', 'P1', 'P2', 'P2', 'P3']),
      模块: [...new Set([pick(modules), ...(rand() > 0.5 ? [pick(modules)] : [])])],
      负责人: pick(people),
      预估工时: Math.round((rand() * 9 + 0.5) * 2) / 2,
      进度: progress,
      开始日期: status === '待评估' ? undefined : start.format('YYYY-MM-DD'),
      截止日期: status === '待评估' && rand() > 0.5 ? undefined : end.format('YYYY-MM-DD'),
      已评审: status !== '待评估' && rand() > 0.2,
      备注: i % 5 === 0 ? '需要与设计同学对齐交互细节，关注边界情况与异常提示。' : undefined,
    }
  })
  buildTable(
    seed,
    rd.uid,
    {
      name: '需求池',
      fields: [
        { label: '需求标题', type: 'text' },
        {
          label: '状态',
          type: 'single_select',
          options: [
            ['待评估', 9],
            ['待开发', 0],
            ['开发中', 2],
            ['测试中', 4],
            ['已上线', 1],
            ['已搁置', 3],
          ],
          metadata: { default: '' },
        },
        {
          label: '优先级',
          type: 'single_select',
          options: [
            ['P0', 3],
            ['P1', 2],
            ['P2', 0],
            ['P3', 9],
          ],
        },
        {
          label: '模块',
          type: 'multi_select',
          options: [
            ['多维表格', 0],
            ['表单', 5],
            ['权限', 4],
            ['开放平台', 8],
            ['性能', 2],
            ['体验', 6],
          ],
        },
        { label: '负责人', type: 'text' },
        { label: '预估工时', type: 'number', metadata: { format: '0.0', default: null } },
        { label: '进度', type: 'number', metadata: { format: '0%', default: 0 } },
        { label: '开始日期', type: 'datetime' },
        { label: '截止日期', type: 'datetime' },
        { label: '已评审', type: 'checkbox' },
        { label: '剩余天数', type: 'formula', exp: 'IF(ISBLANK({截止日期}), "", DATEDIF(TODAY(), {截止日期}, "D"))' },
        { label: '人力成本', type: 'formula', exp: '{预估工时} * 1200' },
        { label: '备注', type: 'text' },
      ],
      rows: rdRows,
      views: [
        { name: '全部需求', type: 'grid', config: (f) => ({ summary: { [f('预估工时')]: 'sum', [f('进度')]: 'average' } }) },
        { name: '按状态看板', type: 'kanban', config: (f) => ({ kanbanFieldUID: f('状态') }) },
        {
          name: '按负责人分组',
          type: 'grid',
          config: (f) => ({
            group: [{ fieldUID: f('负责人'), order: 'asc' }],
            sort: [{ fieldUID: f('优先级'), order: 'asc' }],
          }),
        },
        {
          name: '进行中的 P0/P1',
          type: 'grid',
          config: (f) => ({
            filter: [
              { fieldUID: f('状态'), operation: 'in', value: JSON.stringify([]) },
              { fieldUID: f('优先级'), operation: 'in', value: JSON.stringify([]) },
            ],
          }),
        },
        { name: '需求画册', type: 'gallery' },
        {
          name: '需求收集表',
          type: 'form',
          config: () => ({ form: { title: '需求收集表', description: '欢迎提交你的产品需求，我们会在一周内评估。', fields: [] } }),
        },
      ],
    },
    today.subtract(30, 'day'),
  )
  // 视图筛选里的选项 UID 需要在字段生成后回填。
  {
    const table = seed.tables[seed.tables.length - 1]!
    const fields = seed.fields.filter((f) => f.tableUID === table.uid)
    const opt = (label: string, names: string[]) => {
      const f = fields.find((x) => x.label === label)!
      const opts = (f.metadata as { options: SelectOption[] }).options
      return JSON.stringify(names.map((n) => opts.find((o) => o.name === n)!.uid))
    }
    const view = seed.views.find((v) => v.tableUID === table.uid && v.name === '进行中的 P0/P1')!
    view.config.filter[0]!.value = opt('状态', ['开发中', '测试中'])
    view.config.filter[1]!.value = opt('优先级', ['P0', 'P1'])
  }

  const bugTitles = [
    '筛选后统计栏数值不刷新',
    '拖拽列宽后刷新丢失',
    '看板卡片拖到空列报错',
    '日期选择器在 Safari 下错位',
    '公式引用被删除字段时未提示',
    '粘贴含换行的文本被拆成多行',
    '分组折叠后滚动条跳动',
    '长文本在超高行高下未换行',
    '表单必填项未校验',
    '选项重命名后筛选失效',
    '冻结列阴影在暗色模式下不可见',
    '批量删除后选中状态残留',
    '撤销操作后单元格未聚焦',
    '搜索高亮与选中框重叠',
    '数字字段输入负数显示异常',
    '复制数据表后公式引用错乱',
    '中文输入法下回车直接提交',
    '列头菜单超出屏幕边界',
  ]
  buildTable(
    seed,
    rd.uid,
    {
      name: '缺陷跟踪',
      fields: [
        { label: '缺陷描述', type: 'text' },
        {
          label: '严重程度',
          type: 'single_select',
          options: [
            ['致命', 3],
            ['严重', 2],
            ['一般', 0],
            ['提示', 9],
          ],
        },
        {
          label: '处理状态',
          type: 'single_select',
          options: [
            ['新建', 0],
            ['处理中', 2],
            ['已解决', 1],
            ['已关闭', 9],
            ['重新打开', 3],
          ],
        },
        { label: '处理人', type: 'text' },
        { label: '发现版本', type: 'text' },
        { label: '发现时间', type: 'datetime', metadata: { format: 'YYYY-MM-DD', with_time: true, default: 'now' } },
        { label: '可复现', type: 'checkbox' },
      ],
      rows: bugTitles.map((t) => ({
        缺陷描述: t,
        严重程度: pick(['致命', '严重', '严重', '一般', '一般', '一般', '提示']),
        处理状态: pick(['新建', '处理中', '处理中', '已解决', '已关闭', '重新打开']),
        处理人: pick(people),
        发现版本: pick(['v1.2.0', 'v1.2.1', 'v1.3.0-beta']),
        发现时间: today
          .subtract(Math.floor(rand() * 14), 'day')
          .hour(9 + Math.floor(rand() * 9))
          .minute(Math.floor(rand() * 60))
          .toISOString(),
        可复现: rand() > 0.3,
      })),
      views: [
        { name: '全部缺陷', type: 'grid' },
        { name: '处理看板', type: 'kanban', config: (f) => ({ kanbanFieldUID: f('处理状态') }) },
      ],
    },
    today.subtract(20, 'day'),
  )

  buildTable(
    seed,
    rd.uid,
    {
      name: '迭代计划',
      fields: [
        { label: '迭代', type: 'text' },
        { label: '开始', type: 'datetime' },
        { label: '结束', type: 'datetime' },
        { label: '目标', type: 'text' },
        { label: '完成率', type: 'number', metadata: { format: '0%', default: null } },
        { label: '周期（天）', type: 'formula', exp: 'DATEDIF({开始}, {结束}, "D") + 1' },
        { label: '是否结束', type: 'formula', exp: 'IF({结束} < TODAY(), "已结束", "进行中")' },
      ],
      rows: Array.from({ length: 6 }, (_, i) => {
        const start = today.subtract(56 - i * 14, 'day')
        return {
          迭代: `Sprint ${i + 18}`,
          开始: start.format('YYYY-MM-DD'),
          结束: start.add(13, 'day').format('YYYY-MM-DD'),
          目标: ['上线看板视图', '性能优化', '表单视图 2.0', '公式能力增强', '开放 API', '移动端适配'][i],
          完成率: i < 4 ? 1 - i * 0.05 : i === 4 ? 0.45 : 0,
        }
      }),
      views: [{ name: '迭代列表', type: 'grid' }],
    },
    today.subtract(18, 'day'),
  )

  // ---- 项目二：客户管理 ----
  const crm = mkProject('客户管理 CRM', 'crm', 12)
  const companies = [
    '星河科技',
    '云帆数据',
    '青禾教育',
    '远山物流',
    '拾光文化',
    '北辰医疗',
    '海岚零售',
    '知行咨询',
    '微光智能',
    '锦程制造',
    '蓝鲸金融',
    '晨曦传媒',
    '森林餐饮',
    '天工建筑',
    '若水能源',
    '众合电商',
    '极客互动',
    '百川农业',
  ]
  buildTable(
    seed,
    crm.uid,
    {
      name: '客户列表',
      fields: [
        { label: '公司名称', type: 'text' },
        {
          label: '行业',
          type: 'single_select',
          options: [
            ['互联网', 0],
            ['教育', 5],
            ['医疗', 3],
            ['零售', 2],
            ['制造', 9],
            ['金融', 7],
            ['文化传媒', 6],
          ],
        },
        {
          label: '客户阶段',
          type: 'single_select',
          options: [
            ['线索', 9],
            ['初步接触', 0],
            ['需求确认', 4],
            ['商务谈判', 2],
            ['已签约', 1],
            ['已流失', 3],
          ],
        },
        { label: '城市', type: 'text' },
        { label: '联系人', type: 'text' },
        { label: '年合同额', type: 'number', metadata: { format: '¥0,000.00', default: null } },
        { label: '签约日期', type: 'datetime' },
        {
          label: '标签',
          type: 'multi_select',
          options: [
            ['重点客户', 3],
            ['老客户', 1],
            ['转介绍', 4],
            ['大客户', 7],
          ],
        },
        { label: '需续约', type: 'checkbox' },
        { label: '税后金额', type: 'formula', exp: 'ROUND({年合同额} / 1.06, 2)' },
      ],
      rows: companies.map((c) => {
        const stage = pick(['线索', '初步接触', '需求确认', '商务谈判', '已签约', '已签约', '已流失'])
        return {
          公司名称: c,
          行业: pick(['互联网', '教育', '医疗', '零售', '制造', '金融', '文化传媒']),
          客户阶段: stage,
          城市: pick(['北京', '上海', '深圳', '杭州', '成都', '武汉', '南京']),
          联系人: pick(['陈经理', '刘总', '林女士', '黄先生', '吴总监', '郑经理']),
          年合同额: stage === '已签约' || stage === '商务谈判' ? Math.round(rand() * 90 + 10) * 10000 : undefined,
          签约日期: stage === '已签约' ? today.subtract(Math.floor(rand() * 300), 'day').format('YYYY-MM-DD') : undefined,
          标签: rand() > 0.5 ? [pick(['重点客户', '老客户', '转介绍', '大客户'])] : [],
          需续约: stage === '已签约' && rand() > 0.5,
        }
      }),
      views: [
        { name: '全部客户', type: 'grid', config: (f) => ({ summary: { [f('年合同额')]: 'sum' } }) },
        { name: '销售漏斗', type: 'kanban', config: (f) => ({ kanbanFieldUID: f('客户阶段') }) },
        { name: '客户卡片', type: 'gallery' },
      ],
    },
    today.subtract(12, 'day'),
  )

  // ---- 项目三：读书清单 ----
  const books = mkProject('读书清单', 'reading', 3)
  buildTable(
    seed,
    books.uid,
    {
      name: '书单',
      fields: [
        { label: '书名', type: 'text' },
        { label: '作者', type: 'text' },
        {
          label: '分类',
          type: 'multi_select',
          options: [
            ['技术', 0],
            ['设计', 6],
            ['管理', 2],
            ['文学', 4],
            ['历史', 7],
          ],
        },
        {
          label: '阅读状态',
          type: 'single_select',
          options: [
            ['想读', 9],
            ['在读', 0],
            ['读完', 1],
          ],
        },
        { label: '评分', type: 'number', metadata: { format: '0.0', default: null } },
        { label: '读完日期', type: 'datetime', metadata: { format: 'YYYY年MM月DD日', with_time: false, default: '' } },
      ],
      rows: [
        ['设计数据密集型应用', 'Martin Kleppmann', ['技术'], '读完', 9.6],
        ['人月神话', 'Frederick Brooks', ['技术', '管理'], '读完', 8.8],
        ['设计心理学', 'Don Norman', ['设计'], '在读', null],
        ['重构', 'Martin Fowler', ['技术'], '读完', 9.1],
        ['百年孤独', '加西亚·马尔克斯', ['文学'], '想读', null],
        ['万历十五年', '黄仁宇', ['历史'], '读完', 9.0],
        ['高效能人士的七个习惯', '史蒂芬·柯维', ['管理'], '想读', null],
        ['写给大家看的设计书', 'Robin Williams', ['设计'], '在读', null],
      ].map(([name, author, cat, status, score], i) => ({
        书名: name,
        作者: author,
        分类: cat,
        阅读状态: status,
        评分: score,
        读完日期: status === '读完' ? today.subtract(20 + i * 17, 'day').format('YYYY-MM-DD') : undefined,
      })),
      views: [
        { name: '全部书籍', type: 'grid' },
        { name: '阅读进度', type: 'kanban', config: (f) => ({ kanbanFieldUID: f('阅读状态') }) },
      ],
    },
    today.subtract(3, 'day'),
  )

  return seed
}
