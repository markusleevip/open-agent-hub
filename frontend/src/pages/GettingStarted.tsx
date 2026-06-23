import { useMemo } from 'react'
import {
  Typography,
  Card,
  Alert,
  Steps,
  Tabs,
  Table,
  Tag,
  Space,
  Button,
  Divider
} from 'antd'
import {
  RocketOutlined,
  KeyOutlined,
  ApiOutlined,
  CheckCircleOutlined,
  ThunderboltOutlined,
  ReadOutlined,
  DatabaseOutlined,
  ToolOutlined,
  SafetyCertificateOutlined
} from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import { useAuthStore } from '@/store/auth'
import { useTranslation } from 'react-i18next'

const { Title, Paragraph, Text, Link } = Typography

// Code block with copy button
const CodeBlock = ({ code }: { code: string }) => (
  <div style={{ position: 'relative' }}>
    <Text
      copyable={{ text: code }}
      style={{ position: 'absolute', top: 8, right: 12, zIndex: 1 }}
    />
    <pre className="json-block" style={{ marginBottom: 0 }}>
      {code}
    </pre>
  </div>
)

const GettingStartedPage = () => {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { workspace, user } = useAuthStore()

  // MCP Gateway default port differs from Console (8085). Replace with your gateway URL in production.
  const mcpUrl = useMemo(
    () => `${window.location.protocol}//${window.location.hostname}:8085/mcp`,
    []
  )

  const tokenPlaceholder = t('guide.tokenPlaceholder')
  const projectPathPlaceholder = t('guide.projectPathPlaceholder')
  const projectNamePlaceholder = t('guide.projectNamePlaceholder')

  const cursorConfig = `{
  "mcpServers": {
    "open-agent-hub": {
      "url": "${mcpUrl}",
      "headers": {
        "Authorization": "Bearer ${tokenPlaceholder}"
      }
    }
  }
}`

  const claudeConfig = `{
  "mcpServers": {
    "open-agent-hub": {
      "type": "http",
      "url": "${mcpUrl}",
      "headers": {
        "Authorization": "Bearer ${tokenPlaceholder}"
      }
    }
  }
}`

  const opencodeConfig = `{
  "mcp": {
    "open-agent-hub": {
      "type": "remote",
      "url": "${mcpUrl}",
      "headers": {
        "Authorization": "Bearer ${tokenPlaceholder}"
      },
      "enabled": true
    }
  }
}`

  const curlVerify = `curl -X POST ${mcpUrl} \\
  -H "Authorization: Bearer ${tokenPlaceholder}" \\
  -H "Content-Type: application/json" \\
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}'`

  const syncProjectCall = `hub.sync_project(
  project_path=${projectPathPlaceholder},
  register_project=true,
  project_name=${projectNamePlaceholder}
)`

  const toolRows = [
    { group: t('guide.rulesTitle'), name: 'hub.get_global_rules', desc: t('guide.rule1Desc') },
    { group: t('guide.rulesTitle'), name: 'hub.get_project_rules', desc: t('guide.rule2Desc') },
    { group: t('guide.rulesTitle'), name: 'hub.get_project_context', desc: t('guide.rule3Desc') },
    { group: t('guide.rulesTitle'), name: 'hub.get_workspace_policy', desc: t('guide.rule4Desc') },
    { group: t('guide.memoryTitle'), name: 'hub.search_memory', desc: t('guide.mem1Desc') },
    { group: t('guide.memoryTitle'), name: 'hub.get_relevant_memory', desc: t('guide.mem2Desc') },
    { group: t('guide.memoryTitle'), name: 'hub.propose_memory', desc: t('guide.mem3Desc') },
    { group: t('guide.memoryTitle'), name: 'hub.update_memory / archive_memory', desc: t('guide.mem4Desc') },
    { group: t('guide.toolsTitle'), name: 'hub.list_connected_tools', desc: t('guide.conn1Desc') },
    { group: t('guide.toolsTitle'), name: 'hub.invoke_connected_tool', desc: t('guide.conn2Desc') },
    { group: t('audit.title'), name: 'hub.report_action', desc: t('guide.auditDesc') }
  ]

  return (
    <div className="page-container">
      <Card
        title={
          <Space>
            <RocketOutlined style={{ color: '#1677ff' }} />
            <Title level={4} style={{ margin: 0 }}>
              {t('guide.title')}
            </Title>
          </Space>
        }
      >
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 20 }}
          message={t('guide.whatIsOAH')}
          description={
            <Paragraph style={{ margin: 0 }}>
              {t('guide.descPrefix')}
              <Text strong>{t('guide.descRules')}</Text>、{t('guide.descMemory')}、{t('guide.descSkill')}、{t('guide.descTools')}{t('guide.descSuffix')}
              {user && workspace && (
                <>
                  {' '}{t('guide.currentLogin')}<Text code>{user.username}</Text>{t('guide.workspaceLabel')}{' '}
                  <Text code>{workspace.name}</Text>{t('guide.sentenceEnd')}
                </>
              )}
            </Paragraph>
          }
        />

        <Title level={5}>{t('guide.threeSteps')}</Title>
        <Steps
          direction="vertical"
          current={-1}
          style={{ marginTop: 12 }}
          items={[
            {
              status: 'process',
              icon: <KeyOutlined />,
              title: t('guide.step1Title'),
              description: (
                <div style={{ paddingBottom: 12 }}>
                  <Paragraph>
                    {t('guide.step1Desc1')}<Text strong>{t('layout.mcpTokens')}</Text>{t('guide.step1Desc2')}<Text code>pat_</Text>{t('guide.step1Desc3')}
                  </Paragraph>
                  <Button type="primary" ghost icon={<KeyOutlined />} onClick={() => navigate('/tokens')}>
                    {t('guide.step1Button')}
                  </Button>
                </div>
              )
            },
            {
              status: 'process',
              icon: <ApiOutlined />,
              title: t('guide.step2Title'),
              description: (
                <div style={{ paddingBottom: 12 }}>
                  <Paragraph>
                    {t('guide.step2Desc1')}<Text code>{tokenPlaceholder}</Text>{t('guide.step2Desc2')}<Text code>{mcpUrl}</Text>{t('guide.step2Desc3')}<Text code>/mcp</Text>{t('guide.step2Desc4')}
                  </Paragraph>
                  <Tabs
                    items={[
                      {
                        key: 'cursor',
                        label: 'Cursor',
                        children: (
                          <>
                            <Paragraph type="secondary" style={{ marginBottom: 8 }}>
                              {t('guide.cursorWriteTo')}<Text code>~/.cursor/mcp.json</Text>
                            </Paragraph>
                            <CodeBlock code={cursorConfig} />
                          </>
                        )
                      },
                      {
                        key: 'claude',
                        label: 'Claude Code',
                        children: (
                          <>
                            <Paragraph type="secondary" style={{ marginBottom: 8 }}>
                              {t('guide.claudeWriteTo')}<Text code>.mcp.json</Text>{t('guide.claudeWriteToOr')}
                            </Paragraph>
                            <CodeBlock code={claudeConfig} />
                          </>
                        )
                      },
                      {
                        key: 'opencode',
                        label: t('guide.tabOpenCode'),
                        children: (
                          <>
                            <Paragraph type="secondary" style={{ marginBottom: 8 }}>
                              {t('guide.opencodeWriteTo')}<Text code>opencode.json</Text>{t('guide.opencodeWriteToOr')}<Text code>~/.config/opencode/opencode.json</Text>{t('guide.opencodeWriteToOr2')}
                            </Paragraph>
                            <CodeBlock code={opencodeConfig} />
                          </>
                        )
                      },
                      {
                        key: 'other',
                        label: t('guide.tabOther'),
                        children: (
                          <Paragraph style={{ marginBottom: 0 }}>
                            {t('guide.otherDesc1')}<Text strong>Streamable HTTP</Text>{t('guide.otherDesc2')}<Text code>POST {mcpUrl}</Text>{t('guide.otherDesc3')}<Text code>Authorization: Bearer pat_…</Text>{t('guide.otherDesc4')}<Text code>GET /sse</Text> + <Text code>POST /message</Text>{t('guide.otherDesc6')}
                          </Paragraph>
                        )
                      }
                    ]}
                  />
                </div>
              )
            },
            {
              status: 'process',
              icon: <CheckCircleOutlined />,
              title: t('guide.step3Title'),
              description: (
                <div style={{ paddingBottom: 4 }}>
                  <Paragraph>
                    {t('guide.step3Desc1')}<Text code>hub.*</Text>{t('guide.step3Desc2')}<Text code>tools/list</Text>{t('guide.step3Desc3')}
                  </Paragraph>
                  <CodeBlock code={curlVerify} />
                  <Paragraph style={{ marginTop: 8, marginBottom: 0 }}>
                    {t('guide.step3Success1')}<Link onClick={() => navigate('/tool/mcp-inspector')}>{t('guide.mcpInspector')}</Link>{t('guide.step3Success2')}
                  </Paragraph>
                </div>
              )
            }
          ]}
        />

        <Divider />

        <Title level={5}>
          <DatabaseOutlined /> {t('guide.projectSyncTitle')}
        </Title>
        <Alert
          type="success"
          showIcon
          style={{ marginBottom: 16 }}
          message={t('guide.projectSyncAlertTitle')}
          description={t('guide.projectSyncAlertDesc')}
        />
        <Paragraph>
          {t('guide.projectSyncMcpDesc1')}<Text code>hub.sync_project</Text>{t('guide.projectSyncMcpDesc2')}
        </Paragraph>
        <CodeBlock code={syncProjectCall} />
        <Paragraph type="secondary" style={{ marginTop: 8 }}>
          {t('guide.projectSyncMcpNote')}
        </Paragraph>

        <Divider />

        <Title level={5}>
          <ThunderboltOutlined /> {t('guide.whatCanDo')}
        </Title>
        <Space direction="vertical" size="small" style={{ width: '100%', marginBottom: 8 }}>
          <Paragraph style={{ margin: 0 }}>
            <ReadOutlined style={{ color: '#1677ff' }} /> <Text strong>{t('guide.rulesTitle')}</Text>：{t('guide.rulesDesc1')}<Link onClick={() => navigate('/context/global-rules')}>Global Rules</Link>{t('guide.rulesDesc2')}<Link onClick={() => navigate('/context/project-rules')}>Project Rules</Link>{t('guide.rulesDesc3')}
          </Paragraph>
          <Paragraph style={{ margin: 0 }}>
            <SafetyCertificateOutlined style={{ color: '#1677ff' }} /> <Text strong>{t('guide.personalTitle')}</Text>：{t('guide.personalDesc1')}<Link onClick={() => navigate('/context/output-preferences')}>{t('layout.outputPreferences')}</Link>{t('guide.personalDesc2')}<Text code>.openagent/local/profile.md</Text>{t('guide.personalDesc3')}
          </Paragraph>
          <Paragraph style={{ margin: 0 }}>
            <DatabaseOutlined style={{ color: '#1677ff' }} /> <Text strong>{t('guide.memoryTitle')}</Text>：{t('guide.memoryDesc1')}<Text code>propose_memory</Text>{t('guide.memoryDesc2')}<Link onClick={() => navigate('/memory/explorer')}>Memory Explorer</Link>{t('guide.memoryDesc3')}
          </Paragraph>
          <Paragraph style={{ margin: 0 }}>
            <ReadOutlined style={{ color: '#1677ff' }} /> <Text strong>{t('guide.skillsTitle')}</Text>：{t('guide.skillsDesc1')}<Link onClick={() => navigate('/memory/public-skills')}>{t('layout.publicSkills')}</Link>{t('guide.skillsDesc2')}<Text code>.openagent/skills/</Text>{t('guide.skillsDesc3')}
          </Paragraph>
          <Paragraph style={{ margin: 0 }}>
            <ToolOutlined style={{ color: '#1677ff' }} /> <Text strong>{t('guide.toolsTitle')}</Text>：{t('guide.toolsDesc1')}<Link onClick={() => navigate('/tool/connected-servers')}>Connected Servers</Link>{t('guide.toolsDesc2')}
          </Paragraph>
          <Paragraph style={{ margin: 0 }}>
            <SafetyCertificateOutlined style={{ color: '#1677ff' }} /> <Text strong>{t('guide.policyTitle')}</Text>：{t('guide.policyDesc1')}<Link onClick={() => navigate('/tool/policies')}>Tool Policies</Link>{t('guide.policyDesc2')}<Link onClick={() => navigate('/audit')}>{t('audit.title')}</Link>{t('guide.policyDesc3')}
          </Paragraph>
        </Space>

        <Divider />

        <Title level={5}>{t('guide.quickReference')}</Title>
        <Table
          size="small"
          rowKey="name"
          pagination={false}
          dataSource={toolRows}
          columns={[
            {
              title: t('guide.category'),
              dataIndex: 'group',
              width: 120,
              render: (g: string) => <Tag color="blue">{g}</Tag>
            },
            {
              title: t('guide.tool'),
              dataIndex: 'name',
              width: 280,
              render: (n: string) => <Text code>{n}</Text>
            },
            { title: t('guide.usage'), dataIndex: 'desc' }
          ]}
        />
      </Card>
    </div>
  )
}

export default GettingStartedPage
