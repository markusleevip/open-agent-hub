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
  Popconfirm
} from 'antd'
import { PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { connectedServerApi } from '@/api'
import type { ConnectedMCPServer } from '@/types'
import { formatDate, shortId, parseJSONArray, parseJSONObject } from '@/utils'

const { Title, Text } = Typography
const { TextArea } = Input

const STATUS_COLOR: Record<string, string> = {
  active: 'green',
  pending: 'gold',
  error: 'red'
}

const ConnectedServers = () => {
  const { t } = useTranslation()
  const { message } = App.useApp()
  const [data, setData] = useState<ConnectedMCPServer[]>([])
  const [loading, setLoading] = useState(false)
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState<ConnectedMCPServer | null>(null)
  const [form] = Form.useForm()

  const load = async () => {
    setLoading(true)
    try {
      const list = await connectedServerApi.list()
      setData(list)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, [])

  const onNew = () => {
    setEditing(null)
    form.resetFields()
    form.setFieldsValue({
      transport: 'streamable_http',
      auth_type: 'none',
      tools_json: '[]',
      policy_json: '{}'
    })
    setOpen(true)
  }

  const onEdit = (s: ConnectedMCPServer) => {
    setEditing(s)
    form.setFieldsValue({
      ...s,
      tools_json: s.tools_json || '[]',
      policy_json: s.policy_json || '{}'
    })
    setOpen(true)
  }

  const onSubmit = async () => {
    const values = await form.validateFields()
    // Validate tools_json
    try {
      JSON.parse(values.tools_json || '[]')
    } catch {
      message.error(t('tools.connectedServers.errToolsJson'))
      return
    }
    try {
      JSON.parse(values.policy_json || '{}')
    } catch {
      message.error(t('tools.connectedServers.errPolicyJson'))
      return
    }
    if (editing) {
      await connectedServerApi.update(editing.id, values)
      message.success(t('tools.connectedServers.msgUpdated'))
    } else {
      await connectedServerApi.create(values)
      message.success(t('tools.connectedServers.msgCreated'))
    }
    setOpen(false)
    load()
  }

  const onDelete = async (id: string) => {
    await connectedServerApi.delete(id)
    message.success(t('tools.connectedServers.msgDeleted'))
    load()
  }

  return (
    <div className="page-container">
      <Card
        title={<Title level={4} style={{ margin: 0 }}>{t('tools.connectedServers.title')}</Title>}
        extra={
          <Button type="primary" icon={<PlusOutlined />} onClick={onNew}>
            {t('tools.connectedServers.newConnection')}
          </Button>
        }
      >
        <Text type="secondary" style={{ display: 'block', marginBottom: 12 }}>
          {t('tools.connectedServers.subtitle')}
        </Text>
        <Table
          rowKey="id"
          loading={loading}
          dataSource={data}
          pagination={{ pageSize: 20 }}
          columns={[
            { title: t('tools.connectedServers.colId'), dataIndex: 'id', width: 110, render: (v) => <code>{shortId(v)}</code> },
            { title: t('tools.connectedServers.colName'), dataIndex: 'name', width: 140 },
            { title: t('tools.connectedServers.colDisplayName'), dataIndex: 'display_name', width: 160 },
            { title: t('tools.connectedServers.colEndpoint'), dataIndex: 'endpoint', ellipsis: true },
            { title: t('tools.connectedServers.colTransport'), dataIndex: 'transport', width: 130, render: (v) => <Tag>{v}</Tag> },
            { title: t('tools.connectedServers.colAuth'), dataIndex: 'auth_type', width: 90, render: (v) => <Tag>{v}</Tag> },
            {
              title: t('tools.connectedServers.colStatus'),
              dataIndex: 'status',
              width: 90,
              render: (v) => <Tag color={STATUS_COLOR[v] || 'default'}>{v}</Tag>
            },
            {
              title: t('tools.connectedServers.colToolsCount'),
              width: 80,
              render: (_, r) => (parseJSONArray(r.tools_json) || []).length
            },
            {
              title: t('tools.connectedServers.colLastHealthCheck'),
              dataIndex: 'last_health_check_at',
              width: 170,
              render: (v) => formatDate(v)
            },
            {
              title: t('tools.connectedServers.colActions'),
              width: 160,
              render: (_, r) => (
                <Space>
                  <Button type="link" size="small" icon={<EditOutlined />} onClick={() => onEdit(r)}>
                    {t('tools.connectedServers.edit')}
                  </Button>
                  <Popconfirm title={t('tools.connectedServers.confirmDelete')} onConfirm={() => onDelete(r.id)}>
                    <Button type="link" size="small" danger icon={<DeleteOutlined />}>
                      {t('tools.connectedServers.delete')}
                    </Button>
                  </Popconfirm>
                </Space>
              )
            }
          ]}
        />
      </Card>

      <Modal
        title={editing ? t('tools.connectedServers.editTitle') : t('tools.connectedServers.newTitle')}
        open={open}
        onCancel={() => setOpen(false)}
        onOk={onSubmit}
        destroyOnClose
        width={760}
      >
        <Form form={form} layout="vertical">
          <Form.Item name="name" label={t('tools.connectedServers.labelName')} rules={[{ required: true }]}>
            <Input placeholder={t('tools.connectedServers.placeholderName')} />
          </Form.Item>
          <Form.Item name="display_name" label={t('tools.connectedServers.labelDisplayName')}>
            <Input placeholder={t('tools.connectedServers.placeholderDisplayName')} />
          </Form.Item>
          <Form.Item name="endpoint" label={t('tools.connectedServers.labelEndpoint')} rules={[{ required: true, type: 'url' }]}>
            <Input placeholder="https://api.githubcopilot.com/mcp/" />
          </Form.Item>
          <Space style={{ width: '100%' }}>
            <Form.Item name="transport" label={t('tools.connectedServers.labelTransport')} style={{ width: 200 }}>
              <Select
                options={[
                  { value: 'streamable_http', label: 'streamable_http' },
                  { value: 'sse', label: 'sse' },
                  { value: 'stdio', label: 'stdio' }
                ]}
              />
            </Form.Item>
            <Form.Item name="auth_type" label={t('tools.connectedServers.labelAuth')} style={{ width: 200 }}>
              <Select
                options={[
                  { value: 'none', label: 'none' },
                  { value: 'bearer', label: 'bearer' },
                  { value: 'api_key', label: 'api_key' },
                  { value: 'oauth', label: 'oauth' }
                ]}
              />
            </Form.Item>
          </Space>
          <Form.Item name="tools_json" label={t('tools.connectedServers.labelToolsJson')}>
            <TextArea
              rows={3}
              placeholder='[{"name":"create_issue","description":"...","input_schema":{...}}]'
            />
          </Form.Item>
          <Form.Item name="policy_json" label={t('tools.connectedServers.labelPolicyJson')}>
            <TextArea
              rows={3}
              placeholder='{"require_confirm":["delete_*"],"blocked":["push_main"]}'
            />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export default ConnectedServers
