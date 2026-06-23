import { useState } from 'react'
import {
  Card,
  Form,
  Input,
  Button,
  Select,
  Space,
  Typography,
  App,
  Alert,
  Tabs,
  Tag,
  Empty
} from 'antd'
import { PlayCircleOutlined, CodeOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { mcpApi } from '@/api'

const { Title, Text } = Typography
const { TextArea } = Input

const METHODS = [
  'tools/list',
  'tools/call',
  'initialize',
  'hub.get_agent_profile',
  'hub.get_global_rules',
  'hub.get_workspace_policy',
  'hub.get_output_preferences',
  'hub.get_usage_policy',
  'hub.get_remaining_quota',
  'hub.search_memory',
  'hub.get_relevant_memory',
  'hub.report_action'
]

interface HistoryItem {
  ts: number
  method: string
  ok: boolean
  request: unknown
  response: unknown
}

const MCPInspector = () => {
  const { t } = useTranslation()
  const { message } = App.useApp()
  const [patToken, setPatToken] = useState('')
  const [method, setMethod] = useState('tools/list')
  const [params, setParams] = useState('{\n  \n}')
  const [resp, setResp] = useState<unknown>(null)
  const [loading, setLoading] = useState(false)
  const [history, setHistory] = useState<HistoryItem[]>([])

  const onSend = async () => {
    let parsed: unknown = {}
    if (params.trim() && params.trim() !== '{}') {
      try {
        parsed = JSON.parse(params)
      } catch (e: any) {
        message.error(t('tools.mcpInspector.errInvalidJson') + e.message)
        return
      }
    }
    // Gateway accepts both PAT (pat_xxx) and Console login JWT:
    // use PAT if provided (simulates a real agent), otherwise fall back to current user's JWT.
    const bearer = patToken.trim() || localStorage.getItem('oah_token')
    if (!bearer) {
      message.error(t('tools.mcpInspector.errNoToken'))
      return
    }
    const tokenStr = patToken.trim() ? 'PAT (pat_…)' : 'JWT (current login user)'
    setLoading(true)
    try {
      const result = await mcpApi.call(bearer, {
        jsonrpc: '2.0',
        id: Date.now(),
        method,
        params: parsed
      })
      setResp(result)
      setHistory((h) => [
        { ts: Date.now(), method, ok: !result?.error, request: { method, params: parsed, token: tokenStr }, response: result },
        ...h
      ].slice(0, 20))
      if (result?.error) {
        message.warning(`${t('tools.mcpInspector.errReturn')}${result.error.message || JSON.stringify(result.error)}`)
      } else {
        message.success(t('tools.mcpInspector.msgSent'))
      }
    } catch (e: any) {
      message.error(e?.message || t('tools.mcpInspector.msgFailed'))
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="page-container">
      <Card title={<Title level={4} style={{ margin: 0 }}>{t('tools.mcpInspector.title')}</Title>}>
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 12 }}
          message={t('tools.mcpInspector.alertMsg')}
        />
        <Form layout="vertical">
          <Space.Compact style={{ width: '100%' }}>
            <Form.Item label={t('tools.mcpInspector.method')} style={{ width: 280, marginRight: 8 }}>
              <Select
                showSearch
                value={method}
                onChange={setMethod}
                options={METHODS.map((m) => ({ value: m, label: m }))}
              />
            </Form.Item>
            <Form.Item label={t('tools.mcpInspector.patLabel')} style={{ width: 320 }}>
              <Input.Password
                placeholder={t('tools.mcpInspector.patPlaceholder')}
                value={patToken}
                onChange={(e) => setPatToken(e.target.value)}
              />
            </Form.Item>
          </Space.Compact>
          <Form.Item label={t('tools.mcpInspector.paramsLabel')}>
            <TextArea rows={6} value={params} onChange={(e) => setParams(e.target.value)} />
          </Form.Item>
          <Button type="primary" icon={<PlayCircleOutlined />} loading={loading} onClick={onSend}>
            {t('tools.mcpInspector.btnSend')}
          </Button>
        </Form>
      </Card>

      <Card title={t('tools.mcpInspector.respTitle')} style={{ marginTop: 16 }}>
        {resp == null ? (
          <Empty description={t('tools.mcpInspector.noResp')} />
        ) : (
          <pre style={{ background: '#f5f5f5', padding: 12, borderRadius: 4, overflow: 'auto' }}>
            {JSON.stringify(resp, null, 2)}
          </pre>
        )}
      </Card>

      <Card title={<><CodeOutlined /> {t('tools.mcpInspector.historyTitle')}</>} style={{ marginTop: 16 }}>
        {history.length === 0 ? (
          <Empty description={t('tools.mcpInspector.noHistory')} />
        ) : (
          <Tabs
            items={history.map((h, idx) => ({
              key: String(idx),
              label: (
                <Space>
                  {h.ok ? <Tag color="green">OK</Tag> : <Tag color="red">ERR</Tag>}
                  <code>{h.method}</code>
                  <Text type="secondary" style={{ fontSize: 12 }}>
                    {new Date(h.ts).toLocaleTimeString()}
                  </Text>
                </Space>
              ),
              children: (
                <div>
                  <p><b>Request:</b></p>
                  <pre style={{ background: '#f5f5f5', padding: 12, borderRadius: 4 }}>
                    {JSON.stringify(h.request, null, 2)}
                  </pre>
                  <p><b>Response:</b></p>
                  <pre style={{ background: '#f5f5f5', padding: 12, borderRadius: 4 }}>
                    {JSON.stringify(h.response, null, 2)}
                  </pre>
                </div>
              )
            }))}
          />
        )}
      </Card>
    </div>
  )
}

export default MCPInspector
