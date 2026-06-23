import { useEffect, useState } from 'react'
import { Card, Table, Tag, Typography, Space, Button, Popconfirm, App } from 'antd'
import { ReloadOutlined, DeleteOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { agentClientApi } from '@/api'
import type { AgentClient } from '@/types'
import { formatDate, shortId } from '@/utils'

const { Title, Text } = Typography

const clientTypeColor: Record<string, string> = {
  cursor: 'blue',
  claude_code: 'gold',
  windsurf: 'cyan',
  vscode: 'green',
  custom: 'default',
  unknown: 'default'
}

const AgentClientsPage = () => {
  const { t } = useTranslation()
  const { message } = App.useApp()
  const [data, setData] = useState<AgentClient[]>([])
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    load()
  }, [])

  const load = async () => {
    setLoading(true)
    try {
      setData(await agentClientApi.list())
    } finally {
      setLoading(false)
    }
  }

  const onDelete = async (id: string) => {
    await agentClientApi.delete(id)
    message.success(t('agent.clients.deletedSuccess'))
    load()
  }

  return (
    <div className="page-container">
      <Card
        title={<Title level={4} style={{ margin: 0 }}>Agent Clients</Title>}
        extra={
          <Button icon={<ReloadOutlined />} onClick={load} loading={loading}>
            {t('common.refresh')}
          </Button>
        }
      >
        <Text type="secondary" style={{ display: 'block', marginBottom: 12 }}>
          {t('agent.clients.description')}
        </Text>
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
            {
              title: t('agent.clients.columns.type'),
              dataIndex: 'client_type',
              width: 140,
              render: (v) => <Tag color={clientTypeColor[v] ?? 'default'}>{v}</Tag>
            },
            { title: t('agent.clients.columns.name'), dataIndex: 'client_name' },
            { title: t('agent.clients.columns.version'), dataIndex: 'client_version', width: 120 },
            {
              title: t('agent.clients.columns.installPath'),
              dataIndex: 'install_path',
              ellipsis: true
            },
            {
              title: t('agent.clients.columns.status'),
              dataIndex: 'status',
              width: 100,
              render: (v) => <Tag color={v === 'active' ? 'green' : 'default'}>{v}</Tag>
            },
            {
              title: t('agent.clients.columns.firstSeen'),
              dataIndex: 'first_seen_at',
              width: 170,
              render: (v) => formatDate(v)
            },
            {
              title: t('agent.clients.columns.lastActive'),
              dataIndex: 'last_seen_at',
              width: 170,
              render: (v) => formatDate(v)
            },
            {
              title: t('agent.clients.columns.actions'),
              width: 100,
              render: (_, r) => (
                <Popconfirm title={t('agent.clients.confirmDelete')} onConfirm={() => onDelete(r.id)}>
                  <Button type="link" danger icon={<DeleteOutlined />}>
                    {t('agent.clients.delete')}
                  </Button>
                </Popconfirm>
              )
            }
          ]}
        />
      </Card>
    </div>
  )
}

export default AgentClientsPage
