import { useEffect, useState } from 'react'
import {
  Card,
  Table,
  Select,
  Tag,
  Space,
  Typography,
  Input
} from 'antd'
import { useTranslation } from 'react-i18next'
import { toolInvocationApi } from '@/api'
import type { ToolInvocationLog } from '@/types'
import { formatDate, shortId, truncate } from '@/utils'

const { Title, Text } = Typography

const ToolInvocations = () => {
  const { t } = useTranslation()
  const [data, setData] = useState<ToolInvocationLog[]>([])
  const [loading, setLoading] = useState(false)
  const [toolName, setToolName] = useState<string | undefined>()
  const [status, setStatus] = useState<string | undefined>()

  const load = async () => {
    setLoading(true)
    try {
      const list = await toolInvocationApi.list({
        tool_name: toolName,
        status,
        limit: 200
      })
      setData(list)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [toolName, status])

  return (
    <div className="page-container">
      <Card
        title={<Title level={4} style={{ margin: 0 }}>{t('tools.toolInvocations.title')}</Title>}
        extra={
          <Space>
            <Input
              placeholder={t('tools.toolInvocations.placeholderFilter')}
              allowClear
              style={{ width: 200 }}
              value={toolName}
              onChange={(e) => setToolName(e.target.value || undefined)}
            />
            <Select
              allowClear
              placeholder={t('tools.toolInvocations.placeholderStatus')}
              style={{ width: 120 }}
              value={status}
              onChange={(v) => setStatus(v)}
              options={[
                { value: 'success', label: 'success' },
                { value: 'error', label: 'error' }
              ]}
            />
          </Space>
        }
      >
        <Text type="secondary" style={{ display: 'block', marginBottom: 12 }}>
          {t('tools.toolInvocations.subtitle')}
        </Text>
        <Table
          rowKey="id"
          loading={loading}
          dataSource={data}
          pagination={{ pageSize: 20 }}
          columns={[
            {
              title: t('tools.toolInvocations.colTime'),
              dataIndex: 'invoked_at',
              width: 170,
              render: (v) => formatDate(v)
            },
            { title: t('tools.toolInvocations.colTool'), dataIndex: 'tool_name', render: (v) => <code>{v}</code> },
            {
              title: t('tools.toolInvocations.colStatus'),
              dataIndex: 'status',
              width: 90,
              render: (v) => <Tag color={v === 'success' ? 'green' : 'red'}>{v}</Tag>
            },
            { title: t('tools.toolInvocations.colLatency'), dataIndex: 'latency_ms', width: 90, render: (v) => `${v} ms` },
            {
              title: t('tools.toolInvocations.colConfirmed'),
              dataIndex: 'confirmed',
              width: 80,
              render: (v) => (v ? <Tag color="orange">YES</Tag> : '-')
            },
            {
              title: t('tools.toolInvocations.colError'),
              dataIndex: 'error_code',
              width: 110,
              render: (v) => (v ? <Tag color="red">{v}</Tag> : '-')
            },
            {
              title: t('tools.toolInvocations.colSession'),
              dataIndex: 'mcp_session_id',
              width: 110,
              render: (v) => <code>{shortId(v, 8)}</code>
            },
            {
              title: t('tools.toolInvocations.colOutputSummary'),
              dataIndex: 'output_summary',
              ellipsis: true,
              render: (v) => <Text type="secondary">{truncate(v, 80)}</Text>
            }
          ]}
        />
      </Card>
    </div>
  )
}

export default ToolInvocations
