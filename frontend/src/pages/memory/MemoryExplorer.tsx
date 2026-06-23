import { useEffect, useState } from 'react'
import {
  Card,
  Table,
  Button,
  Modal,
  Form,
  Input,
  Select,
  App,
  Tag,
  Space,
  Typography,
  Popconfirm,
  InputNumber,
  Switch,
  Tabs
} from 'antd'
import { PlusOutlined, EditOutlined, DeleteOutlined, EyeOutlined, SearchOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { memoryApi, projectApi } from '@/api'
import type { Memory, Project } from '@/types'
import { formatDate, shortId, parseJSONArray } from '@/utils'

const { Title, Text } = Typography
const { TextArea } = Input

const MEMORY_TYPES = [
  { value: 'fact' },
  { value: 'preference' },
  { value: 'lesson' },
  { value: 'progress' },
  { value: 'context' },
  { value: 'note' }
]

const PROVENANCE_OPTIONS = [
  { value: 'human_curated' },
  { value: 'agent_proposed' },
  { value: 'imported' }
]

const STATE_COLOR: Record<string, string> = {
  active: 'green',
  pending_review: 'gold',
  archived: 'default',
  rejected: 'red'
}

const PROV_COLOR: Record<string, string> = {
  human_curated: 'blue',
  agent_proposed: 'purple',
  imported: 'cyan'
}

const MemoryExplorer = () => {
  const { t } = useTranslation()
  const { message } = App.useApp()
  const [tab, setTab] = useState('active')
  const [data, setData] = useState<Memory[]>([])
  const [projects, setProjects] = useState<Project[]>([])
  const [loading, setLoading] = useState(false)
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState<Memory | null>(null)
  const [detail, setDetail] = useState<Memory | null>(null)
  const [form] = Form.useForm()
  const [searchQuery, setSearchQuery] = useState('')
  const [filters, setFilters] = useState<{
    type?: string
    provenance?: string
    project_id?: string
  }>({})

  const memoryTypes = MEMORY_TYPES.map((o) => ({
    value: o.value,
    label: `${o.value} (${t(`memory.types.${o.value}`)})`
  }))

  const provenanceOptions = PROVENANCE_OPTIONS.map((o) => ({
    value: o.value,
    label: `${o.value} (${t(`memory.provenance.${o.value}`)})`
  }))

  useEffect(() => {
    projectApi.list().then(setProjects).catch(() => undefined)
  }, [])

  useEffect(() => {
    load()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tab, filters, searchQuery])

  const load = async () => {
    setLoading(true)
    try {
      if (searchQuery.trim()) {
        const list = await memoryApi.search({
          query: searchQuery.trim(),
          scope: filters.project_id ? 'project' : undefined,
          type: filters.type
        })
        setData(list)
      } else {
        const list = await memoryApi.list({
          state: tab,
          ...filters
        })
        setData(list)
      }
    } finally {
      setLoading(false)
    }
  }

  const onNew = () => {
    setEditing(null)
    form.resetFields()
    form.setFieldsValue({
      type: 'fact',
      category: 'workspace',
      scope: 'workspace',
      provenance: 'human_curated',
      importance: 0.5,
      pinned: false
    })
    setOpen(true)
  }

  const onEdit = (m: Memory) => {
    setEditing(m)
    form.setFieldsValue({ ...m, tags: parseJSONArray(m.tags).join(', ') })
    setOpen(true)
  }

  const onSubmit = async () => {
    const values = await form.validateFields()
    const tagsArr = String(values.tags || '')
      .split(',')
      .map((s: string) => s.trim())
      .filter(Boolean)
    const payload: Partial<Memory> = {
      ...values,
      tags: JSON.stringify(tagsArr)
    }
    if (editing) {
      await memoryApi.update(editing.id, payload)
      message.success(t('memory.explorer.updated'))
    } else {
      await memoryApi.create(payload)
      message.success(t('memory.explorer.created'))
    }
    setOpen(false)
    load()
  }

  const onArchive = async (id: string) => {
    await memoryApi.archive(id)
    message.success(t('memory.explorer.archived'))
    load()
  }

  const onDelete = async (id: string) => {
    await memoryApi.delete(id)
    message.success(t('memory.explorer.deleted'))
    load()
  }

  return (
    <div className="page-container">
      <Card
        title={<Title level={4} style={{ margin: 0 }}>{t('memory.explorer.title')}</Title>}
        extra={
          <Space>
            <Input.Search
              allowClear
              placeholder={t('memory.explorer.searchPlaceholder')}
              style={{ width: 220 }}
              enterButton={<SearchOutlined />}
              onSearch={(v) => setSearchQuery(v)}
            />
            <Select
              allowClear
              placeholder={t('memory.explorer.placeholderType')}
              style={{ width: 140 }}
              value={filters.type}
              onChange={(v) => setFilters((f) => ({ ...f, type: v }))}
              options={memoryTypes}
            />
            <Select
              allowClear
              placeholder={t('memory.explorer.placeholderProvenance')}
              style={{ width: 160 }}
              value={filters.provenance}
              onChange={(v) => setFilters((f) => ({ ...f, provenance: v }))}
              options={provenanceOptions}
            />
            <Select
              allowClear
              placeholder={t('memory.explorer.placeholderProject')}
              style={{ width: 200 }}
              value={filters.project_id}
              onChange={(v) => setFilters((f) => ({ ...f, project_id: v }))}
              options={projects.map((p) => ({ value: p.id, label: p.name }))}
            />
            <Button type="primary" icon={<PlusOutlined />} onClick={onNew}>
              {t('memory.explorer.newMemory')}
            </Button>
          </Space>
        }
      >
        <Text type="secondary" style={{ display: 'block', marginBottom: 12 }}>
          {t('memory.explorer.description')}
        </Text>
        <Tabs
          activeKey={tab}
          onChange={setTab}
          items={[
            { key: 'active', label: t('memory.state.active') },
            { key: 'pending_review', label: t('memory.state.pending_review') },
            { key: 'archived', label: t('memory.state.archived') },
            { key: 'rejected', label: t('memory.state.rejected') }
          ]}
        />
        <Table
          rowKey="id"
          loading={loading}
          dataSource={data}
          pagination={{ pageSize: 20 }}
          columns={[
            { title: t('memory.explorer.columns.id'), dataIndex: 'id', width: 110, render: (v) => <code>{shortId(v)}</code> },
            { title: t('memory.explorer.columns.content'), dataIndex: 'content', ellipsis: true },
            {
              title: t('memory.explorer.columns.type'),
              dataIndex: 'type',
              width: 100,
              render: (v) => <Tag color="purple">{v}</Tag>
            },
            {
              title: t('memory.explorer.columns.provenance'),
              dataIndex: 'provenance',
              width: 130,
              render: (v) => <Tag color={PROV_COLOR[v] || 'default'}>{v}</Tag>
            },
            {
              title: t('memory.explorer.columns.state'),
              dataIndex: 'state',
              width: 110,
              render: (v) => <Tag color={STATE_COLOR[v] || 'default'}>{v}</Tag>
            },
            {
              title: t('memory.explorer.columns.importance'),
              dataIndex: 'importance',
              width: 80,
              render: (v) => v?.toFixed?.(2) ?? v
            },
            {
              title: t('memory.explorer.columns.access'),
              dataIndex: 'access_count',
              width: 70
            },
            {
              title: t('memory.explorer.columns.pinned'),
              dataIndex: 'pinned',
              width: 60,
              render: (v) => (v ? <Tag color="orange">PIN</Tag> : '-')
            },
            {
              title: t('memory.explorer.columns.updated'),
              dataIndex: 'updated_at',
              width: 170,
              render: (v) => formatDate(v)
            },
            {
              title: t('memory.explorer.columns.actions'),
              width: 260,
              render: (_, r) => (
                <Space size="small">
                  <Button type="link" size="small" icon={<EyeOutlined />} onClick={() => setDetail(r)}>
                    {t('memory.explorer.actions.view')}
                  </Button>
                  <Button type="link" size="small" icon={<EditOutlined />} onClick={() => onEdit(r)}>
                    {t('memory.explorer.actions.edit')}
                  </Button>
                  {r.state !== 'archived' && (
                    <Popconfirm title={t('memory.explorer.actions.confirmArchive')} onConfirm={() => onArchive(r.id)}>
                      <Button type="link" size="small">
                        {t('memory.explorer.actions.archive')}
                      </Button>
                    </Popconfirm>
                  )}
                  <Popconfirm title={t('memory.explorer.actions.confirmDelete')} onConfirm={() => onDelete(r.id)}>
                    <Button type="link" size="small" danger icon={<DeleteOutlined />}>
                      {t('memory.explorer.actions.delete')}
                    </Button>
                  </Popconfirm>
                </Space>
              )
            }
          ]}
        />
      </Card>

      <Modal
        title={editing ? t('memory.explorer.modal.editTitle') : t('memory.explorer.modal.newTitle')}
        open={open}
        onCancel={() => setOpen(false)}
        onOk={onSubmit}
        destroyOnClose
        width={760}
      >
        <Form form={form} layout="vertical">
          <Form.Item name="content" label={t('memory.explorer.form.content')} rules={[{ required: true }]}>
            <TextArea rows={4} placeholder={t('memory.explorer.form.contentPlaceholder')} />
          </Form.Item>
          <Space style={{ width: '100%' }}>
            <Form.Item name="type" label={t('memory.explorer.form.type')} rules={[{ required: true }]} style={{ width: 200 }}>
              <Select options={memoryTypes} />
            </Form.Item>
            <Form.Item name="category" label={t('memory.explorer.form.category')} rules={[{ required: true }]} style={{ width: 200 }}>
              <Select
                options={[
                  { value: 'workspace', label: 'workspace' },
                  { value: 'project', label: 'project' }
                ]}
              />
            </Form.Item>
            <Form.Item name="provenance" label={t('memory.explorer.form.provenance')} style={{ width: 200 }}>
              <Select options={provenanceOptions} />
            </Form.Item>
          </Space>
          <Space style={{ width: '100%' }}>
            <Form.Item name="scope" label={t('memory.explorer.form.scope')} style={{ width: 200 }}>
              <Select
                options={[
                  { value: 'workspace', label: 'workspace' },
                  { value: 'project', label: 'project' }
                ]}
              />
            </Form.Item>
            <Form.Item name="project_id" label={t('memory.explorer.form.projectRequired')} style={{ width: 240 }}>
              <Select
                allowClear
                options={projects.map((p) => ({ value: p.id, label: p.name }))}
              />
            </Form.Item>
            <Form.Item name="importance" label={t('memory.explorer.form.importance')} style={{ width: 160 }}>
              <InputNumber min={0} max={1} step={0.05} style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item name="pinned" label={t('memory.explorer.form.pinned')} valuePropName="checked">
              <Switch />
            </Form.Item>
          </Space>
          <Form.Item name="tags" label={t('memory.explorer.form.tags')}>
            <Input placeholder={t('memory.explorer.form.tagsPlaceholder')} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={t('memory.explorer.modal.detailTitle')}
        open={!!detail}
        onCancel={() => setDetail(null)}
        footer={<Button onClick={() => setDetail(null)}>{t('memory.explorer.actions.close')}</Button>}
        width={720}
      >
        {detail && (
          <div>
            <p><b>{t('memory.explorer.detail.id')}</b><code>{detail.id}</code></p>
            <p><b>{t('memory.explorer.detail.type')}</b><Tag color="purple">{detail.type}</Tag></p>
            <p><b>{t('memory.explorer.detail.provenance')}</b><Tag color={PROV_COLOR[detail.provenance] || 'default'}>{detail.provenance}</Tag></p>
            <p><b>{t('memory.explorer.detail.state')}</b><Tag color={STATE_COLOR[detail.state] || 'default'}>{detail.state}</Tag></p>
            <p><b>{t('memory.explorer.detail.importance')}</b>{detail.importance}</p>
            <p><b>{t('memory.explorer.detail.accessCount')}</b>{detail.access_count}</p>
            <p><b>{t('memory.explorer.detail.charCount')}</b>{detail.char_count}</p>
            <p><b>{t('memory.explorer.detail.version')}</b>{detail.version}</p>
            <p><b>{t('memory.explorer.detail.tags')}</b>{(parseJSONArray(detail.tags) || []).map((t) => <Tag key={t}>{t}</Tag>)}</p>
            <p><b>{t('memory.explorer.detail.content')}</b></p>
            <pre style={{ background: '#f5f5f5', padding: 12, borderRadius: 4, whiteSpace: 'pre-wrap' }}>
              {detail.content}
            </pre>
            <p><b>{t('memory.explorer.detail.created')}</b>{formatDate(detail.created_at)}</p>
            <p><b>{t('memory.explorer.detail.updated')}</b>{formatDate(detail.updated_at)}</p>
            <p><b>{t('memory.explorer.detail.lastAccess')}</b>{formatDate(detail.last_access_at)}</p>
          </div>
        )}
      </Modal>
    </div>
  )
}

export default MemoryExplorer
