import { useEffect, useState } from 'react'
import {
  Card,
  Table,
  Button,
  Modal,
  Form,
  Select,
  Switch,
  InputNumber,
  App,
  Tag,
  Space,
  Typography
} from 'antd'
import { EditOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { toolPolicyApi } from '@/api'
import type { ToolPolicy } from '@/types'
import { shortId } from '@/utils'

const { Title, Text } = Typography

const RISK_COLOR: Record<string, string> = {
  low: 'green',
  medium: 'gold',
  high: 'red'
}

const ToolPolicies = () => {
  const { t } = useTranslation()
  const { message } = App.useApp()
  const [data, setData] = useState<ToolPolicy[]>([])
  const [loading, setLoading] = useState(false)
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState<ToolPolicy | null>(null)
  const [form] = Form.useForm()

  const load = async () => {
    setLoading(true)
    try {
      const list = await toolPolicyApi.list()
      setData(list)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, [])

  const onEdit = (p: ToolPolicy) => {
    setEditing(p)
    form.setFieldsValue(p)
    setOpen(true)
  }

  const onSubmit = async () => {
    const v = await form.validateFields()
    await toolPolicyApi.update(editing!.id, v)
    message.success(t('tools.toolPolicies.msgUpdated'))
    setOpen(false)
    load()
  }

  return (
    <div className="page-container">
      <Card title={<Title level={4} style={{ margin: 0 }}>{t('tools.toolPolicies.title')}</Title>}>
        <Text type="secondary" style={{ display: 'block', marginBottom: 12 }}>
          {t('tools.toolPolicies.subtitle')}
        </Text>
        <Table
          rowKey="id"
          loading={loading}
          dataSource={data}
          pagination={{ pageSize: 20 }}
          columns={[
            { title: 'ID', dataIndex: 'id', width: 110, render: (v) => <code>{shortId(v)}</code> },
            { title: t('tools.toolPolicies.colServer'), dataIndex: 'connected_server_id', width: 110, render: (v) => <code>{shortId(v)}</code> },
            { title: t('tools.toolPolicies.colTool'), dataIndex: 'tool_name', render: (v) => <code>{v}</code> },
            {
              title: t('tools.toolPolicies.colAllowed'),
              dataIndex: 'allowed',
              width: 80,
              render: (v) => (v ? <Tag color="green">YES</Tag> : <Tag color="red">NO</Tag>)
            },
            {
              title: t('tools.toolPolicies.colConfirmed'),
              dataIndex: 'requires_confirmation',
              width: 90,
              render: (v) => (v ? <Tag color="orange">YES</Tag> : <Tag>NO</Tag>)
            },
            { title: t('tools.toolPolicies.colDailyLimit'), dataIndex: 'max_calls_per_day', width: 100 },
            { title: t('tools.toolPolicies.colUserLimit'), dataIndex: 'max_calls_per_user', width: 100 },
            {
              title: t('tools.toolPolicies.colRisk'),
              dataIndex: 'risk_level',
              width: 90,
              render: (v) => <Tag color={RISK_COLOR[v] || 'default'}>{v}</Tag>
            },
            {
              title: t('tools.toolPolicies.colActions'),
              width: 100,
              render: (_, r) => (
                <Button type="link" size="small" icon={<EditOutlined />} onClick={() => onEdit(r)}>
                  {t('tools.toolPolicies.edit')}
                </Button>
              )
            }
          ]}
        />
      </Card>

      <Modal
        title={t('tools.toolPolicies.modalTitle')}
        open={open}
        onCancel={() => setOpen(false)}
        onOk={onSubmit}
        destroyOnClose
        width={620}
      >
        <Form form={form} layout="vertical">
          <Form.Item name="allowed" label={t('tools.toolPolicies.labelAllowed')} valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item name="requires_confirmation" label={t('tools.toolPolicies.labelConfirmed')} valuePropName="checked">
            <Switch />
          </Form.Item>
          <Space style={{ width: '100%' }}>
            <Form.Item name="max_calls_per_day" label={t('tools.toolPolicies.labelDailyLimit')} style={{ width: 200 }}>
              <InputNumber min={0} style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item name="max_calls_per_user" label={t('tools.toolPolicies.labelUserLimit')} style={{ width: 200 }}>
              <InputNumber min={0} style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item name="risk_level" label={t('tools.toolPolicies.labelRiskLevel')} style={{ width: 160 }}>
              <Select
                options={[
                  { value: 'low', label: 'low' },
                  { value: 'medium', label: 'medium' },
                  { value: 'high', label: 'high' }
                ]}
              />
            </Form.Item>
          </Space>
        </Form>
      </Modal>
    </div>
  )
}

export default ToolPolicies
