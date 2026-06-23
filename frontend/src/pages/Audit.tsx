import { useEffect, useState } from 'react'
import { Card, Table, Tag, Typography, Input, Select, Space, Button } from 'antd'
import { ReloadOutlined } from '@ant-design/icons'
import { auditApi } from '@/api'
import type { AuditLog } from '@/types'
import { formatDate, shortId, truncate } from '@/utils'
import { useTranslation } from 'react-i18next'

const { Title, Text } = Typography

const actorColor: Record<string, string> = {
  user: 'blue',
  api_key: 'purple',
  agent: 'green',
  system: 'orange'
}

const AuditPage = () => {
  const { t } = useTranslation()
  const [data, setData] = useState<AuditLog[]>([])
  const [loading, setLoading] = useState(false)
  const [filters, setFilters] = useState<{ action?: string; actor_type?: string; limit?: number }>({
    limit: 100
  })

  useEffect(() => {
    load()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [filters.actor_type, filters.action, filters.limit])

  const load = async () => {
    setLoading(true)
    try {
      const list = await auditApi.list(filters)
      setData(list)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="page-container">
      <Card
        title={<Title level={4} style={{ margin: 0 }}>{t('audit.title')}</Title>}
        extra={
          <Space>
            <Select
              placeholder="actor_type"
              allowClear
              style={{ width: 140 }}
              value={filters.actor_type}
              onChange={(v) => setFilters((f) => ({ ...f, actor_type: v }))}
              options={[
                { value: 'user', label: 'user' },
                { value: 'api_key', label: 'api_key' },
                { value: 'agent', label: 'agent' },
                { value: 'system', label: 'system' }
              ]}
            />
            <Input
              placeholder="action contains..."
              style={{ width: 200 }}
              allowClear
              onPressEnter={(e) =>
                setFilters((f) => ({ ...f, action: (e.target as HTMLInputElement).value || undefined }))
              }
            />
            <Select
              value={filters.limit}
              style={{ width: 100 }}
              onChange={(v) => setFilters((f) => ({ ...f, limit: v }))}
              options={[
                { value: 50, label: t('audit.limit50') },
                { value: 100, label: t('audit.limit100') },
                { value: 500, label: t('audit.limit500') }
              ]}
            />
            <Button icon={<ReloadOutlined />} onClick={load} loading={loading}>
              {t('audit.refresh')}
            </Button>
          </Space>
        }
      >
        <Text type="secondary" style={{ display: 'block', marginBottom: 12 }}>
          {t('audit.desc')}
        </Text>
        <Table
          rowKey="id"
          loading={loading}
          dataSource={data}
          pagination={{ pageSize: 50 }}
          columns={[
            {
              title: t('audit.time'),
              dataIndex: 'created_at',
              width: 170,
              render: (v) => formatDate(v)
            },
            {
              title: 'Actor',
              width: 180,
              render: (_, r) => (
                <Space size={4}>
                  <Tag color={actorColor[r.actor_type] ?? 'default'}>{r.actor_type}</Tag>
                  <code style={{ fontSize: 12 }}>{shortId(r.actor)}</code>
                </Space>
              )
            },
            {
              title: 'Action',
              dataIndex: 'action',
              width: 200,
              render: (v) => <code>{v}</code>
            },
            {
              title: 'Target',
              width: 200,
              render: (_, r) => (
                <Space size={4}>
                  <Tag>{r.target_type}</Tag>
                  <code style={{ fontSize: 12 }}>{shortId(r.target)}</code>
                </Space>
              )
            },
            {
              title: 'IP',
              dataIndex: 'client_ip',
              width: 140
            },
            {
              title: 'Payload',
              dataIndex: 'payload',
              ellipsis: true,
              render: (v) => (
                <Text type="secondary" style={{ fontSize: 12 }}>
                  {truncate(v, 80)}
                </Text>
              )
            }
          ]}
        />
      </Card>
    </div>
  )
}

export default AuditPage
