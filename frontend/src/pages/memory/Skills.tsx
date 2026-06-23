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
  InputNumber,
  Switch,
  Tabs,
  Popconfirm
} from 'antd'
import { PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { skillApi, memoryApi } from '@/api'
import type { Memory } from '@/types'
import { formatDate, shortId, parseJSONArray } from '@/utils'

const { Title, Text } = Typography
const { TextArea } = Input

const STATE_COLOR: Record<string, string> = {
  active: 'green',
  stale: 'gold',
  archived: 'default'
}

const Skills = () => {
  const { t } = useTranslation()
  const { message } = App.useApp()
  const [tab, setTab] = useState('active')
  const [data, setData] = useState<Memory[]>([])
  const [loading, setLoading] = useState(false)
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState<Memory | null>(null)
  const [form] = Form.useForm()

  const load = async (state: string) => {
    setLoading(true)
    try {
      const res = await skillApi.list({ state })
      setData(Array.isArray(res) ? res : [])
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load(tab)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tab])

  const onNew = () => {
    setEditing(null)
    form.resetFields()
    form.setFieldsValue({ importance: 0.7, pinned: false, state: 'active' })
    setOpen(true)
  }

  const onEdit = (m: Memory) => {
    setEditing(m)
    form.setFieldsValue({
      content: m.content,
      importance: m.importance,
      pinned: m.pinned,
      state: m.state,
      tags: parseJSONArray(m.tags).join(', ')
    })
    setOpen(true)
  }

  const onSubmit = async () => {
    const values = await form.validateFields()
    const tagsArr = String(values.tags || '')
      .split(',')
      .map((s: string) => s.trim())
      .filter(Boolean)
    if (editing) {
      const stateChanged = values.state && values.state !== editing.state
      await memoryApi.update(editing.id, {
        content: values.content,
        importance: values.importance,
        pinned: values.pinned,
        tags: JSON.stringify(tagsArr)
      })
      if (stateChanged) {
        await skillApi.changeState(editing.id, values.state)
      }
      message.success(t('memory.skills.updated'))
    } else {
      await skillApi.create({
        content: values.content,
        importance: values.importance,
        pinned: values.pinned,
        tags: JSON.stringify(tagsArr)
      })
      message.success(t('memory.skills.created'))
    }
    setOpen(false)
    load(tab)
  }

  const onDelete = async (id: string) => {
    await memoryApi.delete(id)
    message.success(t('memory.skills.deleted'))
    load(tab)
  }

  return (
    <div className="page-container">
      <Card
        title={<Title level={4} style={{ margin: 0 }}>{t('memory.skills.title')}</Title>}
        extra={
          <Button type="primary" icon={<PlusOutlined />} onClick={onNew}>
            {t('memory.skills.newButton')}
          </Button>
        }
      >
        <Text type="secondary" style={{ display: 'block', marginBottom: 12 }}>
          {t('memory.skills.description')}
        </Text>
        <Tabs
          activeKey={tab}
          onChange={setTab}
          items={[
            { key: 'active', label: 'Active' },
            { key: 'stale', label: 'Stale' },
            { key: 'archived', label: 'Archived' },
            { key: 'all', label: 'All' }
          ]}
        />
        <Table
          rowKey="id"
          loading={loading}
          dataSource={data}
          pagination={{ pageSize: 20 }}
          columns={[
            { title: 'ID', dataIndex: 'id', width: 110, render: (v) => <code>{shortId(v)}</code> },
            { title: t('memory.skills.columns.content'), dataIndex: 'content', ellipsis: true },
            {
              title: t('memory.skills.columns.state'),
              dataIndex: 'state',
              width: 100,
              render: (v) => <Tag color={STATE_COLOR[v] || 'default'}>{v}</Tag>
            },
            {
              title: t('memory.skills.columns.importance'),
              dataIndex: 'importance',
              width: 80,
              render: (v) => v?.toFixed?.(2) ?? v
            },
            {
              title: t('memory.skills.columns.pinned'),
              dataIndex: 'pinned',
              width: 60,
              render: (v) => (v ? <Tag color="orange">PIN</Tag> : '-')
            },
            {
              title: t('memory.skills.columns.tags'),
              dataIndex: 'tags',
              width: 200,
              render: (v) => (
                <Space size={4} wrap>
                  {(parseJSONArray(v) || []).map((t) => (
                    <Tag key={t}>{t}</Tag>
                  ))}
                </Space>
              )
            },
            {
              title: t('memory.skills.columns.charCount'),
              dataIndex: 'char_count',
              width: 80
            },
            {
              title: t('memory.skills.columns.updated'),
              dataIndex: 'updated_at',
              width: 170,
              render: (v) => formatDate(v)
            },
            {
              title: t('memory.skills.columns.actions'),
              width: 150,
              render: (_, r) => (
                <Space size="small">
                  <Button type="link" size="small" icon={<EditOutlined />} onClick={() => onEdit(r)}>
                    {t('memory.skills.editButton')}
                  </Button>
                  <Popconfirm title={t('memory.skills.confirmDelete')} onConfirm={() => onDelete(r.id)}>
                    <Button type="link" size="small" danger icon={<DeleteOutlined />}>
                      {t('memory.skills.deleteButton')}
                    </Button>
                  </Popconfirm>
                </Space>
              )
            }
          ]}
        />
      </Card>

      <Modal
        title={editing ? t('memory.skills.editTitle') : t('memory.skills.newButton')}
        open={open}
        onCancel={() => setOpen(false)}
        onOk={onSubmit}
        destroyOnClose
        width={680}
      >
        <Form form={form} layout="vertical">
          <Form.Item name="content" label={t('memory.skills.form.content')} rules={[{ required: true }]}>
            <TextArea rows={5} placeholder={t('memory.skills.form.contentPlaceholder')} />
          </Form.Item>
          <Space style={{ width: '100%' }}>
            <Form.Item name="importance" label={t('memory.skills.form.importance')} style={{ width: 160 }}>
              <InputNumber min={0} max={1} step={0.05} style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item name="pinned" label={t('memory.explorer.form.pinned')} valuePropName="checked">
              <Switch />
            </Form.Item>
            {editing && (
              <Form.Item name="state" label={t('memory.skills.form.targetState')} style={{ width: 160 }}>
                <Select
                  options={[
                    { value: 'active', label: 'active' },
                    { value: 'stale', label: 'stale' },
                    { value: 'archived', label: 'archived' }
                  ]}
                />
              </Form.Item>
            )}
          </Space>
          <Form.Item name="tags" label={t('memory.skills.form.tags')}>
            <Input placeholder={t('memory.skills.form.tagsPlaceholder')} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export default Skills
