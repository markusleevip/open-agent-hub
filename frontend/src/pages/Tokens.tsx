import { useEffect, useState } from 'react'
import {
  Card,
  Table,
  Button,
  Modal,
  Form,
  Input,
  InputNumber,
  Select,
  App,
  Tag,
  Typography,
  Popconfirm,
  Alert,
  Space
} from 'antd'
import { PlusOutlined, DeleteOutlined, StopOutlined, CopyOutlined } from '@ant-design/icons'
import { tokenApi } from '@/api'
import type { APIKey } from '@/types'
import { formatDate, shortId, copyToClipboard, parseJSONArray } from '@/utils'
import { useAuthStore } from '@/store/auth'
import { useTranslation } from 'react-i18next'

const { Title, Text, Paragraph } = Typography

const TokensPage = () => {
  const { t } = useTranslation()
  const { message } = App.useApp()
  const workspace = useAuthStore((s) => s.workspace)
  const [data, setData] = useState<APIKey[]>([])
  const [loading, setLoading] = useState(false)
  const [open, setOpen] = useState(false)
  const [createdToken, setCreatedToken] = useState<string | null>(null)
  const [form] = Form.useForm()

  useEffect(() => {
    load()
  }, [])

  const load = async () => {
    setLoading(true)
    try {
      const list = await tokenApi.list()
      setData(list)
    } finally {
      setLoading(false)
    }
  }

  const onNew = () => {
    form.resetFields()
    setOpen(true)
  }

  const onSubmit = async () => {
    const values = await form.validateFields()
    const res = await tokenApi.create({
      name: values.name,
      scopes: values.scopes,
      expires_in_days: values.expires_in_days
    })
    setCreatedToken(res.token)
    setOpen(false)
    load()
  }

  const onRevoke = async (id: string) => {
    await tokenApi.revoke(id)
    message.success(t('tokens.revoked'))
    load()
  }

  const onDelete = async (id: string) => {
    await tokenApi.delete(id)
    message.success(t('tokens.deleted'))
    load()
  }

  return (
    <div className="page-container">
      <Card
        title={<Title level={4} style={{ margin: 0 }}>{t('tokens.title')}</Title>}
        extra={
          <Button type="primary" icon={<PlusOutlined />} onClick={onNew}>
            {t('tokens.create')}
          </Button>
        }
      >
        <Alert
          message={t('tokens.alertMsg')}
          description={
            <Space direction="vertical" size={4}>
              <Text>
                <Text code>Authorization: Bearer pat_XXXXXXXX...</Text>
              </Text>
              <Text type="secondary">
                {t('tokens.alertDesc')}
              </Text>
              <Text type="warning" style={{ fontSize: 12 }}>
                {t('tokens.workspaceBind', { name: workspace?.name ?? '-' })}
              </Text>
            </Space>
          }
          type="info"
          showIcon
          style={{ marginBottom: 16 }}
        />
        <Table
          rowKey="id"
          loading={loading}
          dataSource={data}
          pagination={{ pageSize: 20 }}
          columns={[
            {
              title: 'ID',
              dataIndex: 'id',
              width: 110,
              render: (v) => <code>{shortId(v)}</code>
            },
            { title: t('tokens.name'), dataIndex: 'name' },
            {
              title: t('tokens.prefix'),
              dataIndex: 'prefix',
              width: 130,
              render: (v) => <code>{v}…</code>
            },
            {
              title: t('tokens.scopes'),
              dataIndex: 'scopes',
              render: (v: string) => (
                <Space size={4} wrap>
                  {parseJSONArray(v).map((s) => (
                    <Tag key={s} color="blue">
                      {s}
                    </Tag>
                  ))}
                </Space>
              )
            },
            {
              title: t('tokens.lastUsed'),
              dataIndex: 'last_used_at',
              width: 170,
              render: (v) => formatDate(v)
            },
            {
              title: t('tokens.expiresAt'),
              dataIndex: 'expires_at',
              width: 170,
              render: (v) => formatDate(v)
            },
            {
              title: t('tokens.status'),
              width: 90,
              render: (_, r) =>
                r.revoked_at ? (
                  <Tag color="red">revoked</Tag>
                ) : r.expires_at && new Date(r.expires_at) < new Date() ? (
                  <Tag color="orange">expired</Tag>
                ) : (
                  <Tag color="green">active</Tag>
                )
            },
            {
              title: t('tokens.actions'),
              width: 200,
              render: (_, r) => (
                <Space>
                  {!r.revoked_at && (
                    <Popconfirm title={t('tokens.confirmRevoke')} onConfirm={() => onRevoke(r.id)}>
                      <Button type="link" icon={<StopOutlined />}>
                        {t('tokens.revoke')}
                      </Button>
                    </Popconfirm>
                  )}
                  <Popconfirm title={t('tokens.confirmDelete')} onConfirm={() => onDelete(r.id)}>
                    <Button type="link" danger icon={<DeleteOutlined />}>
                      {t('tokens.delete')}
                    </Button>
                  </Popconfirm>
                </Space>
              )
            }
          ]}
        />
      </Card>

      <Modal
        title={t('tokens.generateNewToken')}
        open={open}
        onCancel={() => setOpen(false)}
        onOk={onSubmit}
        destroyOnClose
        width={520}
      >
        <Form
          form={form}
          layout="vertical"
          initialValues={{ scopes: ['read', 'write'], expires_in_days: 30 }}
        >
          <Form.Item name="name" label={t('tokens.name')} rules={[{ required: true, message: t('tokens.name') }]}>
            <Input placeholder={t('tokens.namePlaceholder')} />
          </Form.Item>
          <Form.Item name="scopes" label={t('tokens.scopes')} rules={[{ required: true }]}>
            <Select
              mode="multiple"
              options={[
                { value: 'read', label: t('tokens.scopeRead') },
                { value: 'write', label: t('tokens.scopeWrite') },
                { value: 'admin', label: t('tokens.scopeAdmin') }
              ]}
            />
          </Form.Item>
          <Form.Item name="expires_in_days" label={t('tokens.expiresInDays')}>
            <InputNumber min={0} max={3650} style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={t('tokens.tokenCreated')}
        open={!!createdToken}
        onCancel={() => setCreatedToken(null)}
        onOk={() => setCreatedToken(null)}
        okText={t('tokens.iHaveSaved')}
        cancelButtonProps={{ style: { display: 'none' } }}
        width={620}
      >
        <Alert
          message={t('tokens.saveTokenAlert')}
          type="warning"
          showIcon
          style={{ marginBottom: 12 }}
        />
        <Paragraph copyable={{ text: createdToken ?? '' }}>
          <code style={{ wordBreak: 'break-all', fontSize: 12 }}>{createdToken}</code>
        </Paragraph>
        <Text type="secondary" style={{ fontSize: 12 }}>
          {t('tokens.usageInstructionsPrefix')}
          <code> POST http://&lt;mcp-host&gt;/mcp </code>
          {t('tokens.usageInstructionsSuffix')}
        </Text>
        <Button
          type="link"
          icon={<CopyOutlined />}
          onClick={() => {
            copyToClipboard(createdToken ?? '')
              .then(() => message.success(t('tokens.copied')))
              .catch(() => message.error(t('tokens.copyFailed')))
          }}
        >
          {t('tokens.copy')}
        </Button>
      </Modal>
    </div>
  )
}

export default TokensPage
