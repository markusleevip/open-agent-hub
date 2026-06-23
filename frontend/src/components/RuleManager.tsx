import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
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
  Typography,
  Popconfirm,
  Space,
  InputNumber
} from 'antd'
import { PlusOutlined, DeleteOutlined, EditOutlined } from '@ant-design/icons'
import { ruleApi, projectApi } from '@/api'
import type { Rule, Project } from '@/types'
import { formatDate, shortId, parseJSONArray } from '@/utils'

const { Title, Text } = Typography
const { TextArea } = Input

export interface RuleManagerProps {
  title: string
  description: string
  scope: 'workspace' | 'project'
  /** Enforced type; when omitted the user can choose */
  fixedType?: string
  /** Candidate type list; only used when fixedType is not set */
  typeOptions?: { value: string; label: string }[]
  /** Whether to show a project selector (usually needed for scope=project) */
  withProject?: boolean
  /** Lock to a specific project: hide selector, always load this project's rules */
  lockedProjectId?: string
}

const RuleManager = ({
  title,
  description,
  scope,
  fixedType,
  typeOptions,
  withProject = false,
  lockedProjectId
}: RuleManagerProps) => {
  const { t } = useTranslation()

  const defaultTypes = [
    { value: 'global_rule', label: t('ruleManager.typeGlobalRule') },
    { value: 'project_rule', label: t('ruleManager.typeProjectRule') },
    { value: 'workspace_policy', label: t('ruleManager.typeWorkspacePolicy') },
    { value: 'output_preference', label: t('ruleManager.typeOutputPreference') },
    { value: 'agent_rule', label: t('ruleManager.typeAgentRule') }
  ]

  const resolvedTypeOptions = typeOptions ?? defaultTypes
  const { message } = App.useApp()
  const [data, setData] = useState<Rule[]>([])
  const [projects, setProjects] = useState<Project[]>([])
  const [loading, setLoading] = useState(false)
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState<Rule | null>(null)
  const [form] = Form.useForm()
  const [projectId, setProjectId] = useState<string | undefined>(lockedProjectId)

  useEffect(() => {
    // Skip project list in locked mode
    if (withProject && !lockedProjectId) {
      projectApi.list().then((ps) => {
        setProjects(ps)
        if (ps.length > 0 && !projectId) {
          setProjectId(ps[0].id)
        }
      })
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  useEffect(() => {
    load()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [projectId])

  const load = async () => {
    setLoading(true)
    try {
      let list: Rule[] = []
      if (withProject && projectId) {
        // Use dedicated endpoint for project rules
        list = await ruleApi.projectRules(projectId)
        if (fixedType) {
          list = list.filter((r) => r.type === fixedType)
        }
      } else {
        const params: Record<string, string | undefined> = { scope }
        if (fixedType) {
          params.type = fixedType
        }
        list = await ruleApi.list(params)
      }
      setData(list)
    } finally {
      setLoading(false)
    }
  }

  const onNew = () => {
    setEditing(null)
    form.resetFields()
    form.setFieldsValue({
      scope,
      type: fixedType ?? resolvedTypeOptions[0]?.value,
      project_id: withProject ? projectId : undefined,
      version: 1
    })
    setOpen(true)
  }

  const onEdit = (r: Rule) => {
    setEditing(r)
    form.setFieldsValue({
      ...r,
      tags: parseJSONArray(r.tags).join(', ')
    })
    setOpen(true)
  }

  const onSubmit = async () => {
    const values = await form.validateFields()
    const tagsArr = String(values.tags || '')
      .split(',')
      .map((s: string) => s.trim())
      .filter(Boolean)
    const payload: Partial<Rule> = {
      ...values,
      scope,
      type: fixedType ?? values.type,
      tags: JSON.stringify(tagsArr)
    }
    if (withProject && projectId) {
      payload.project_id = projectId
    }
    if (editing) {
      await ruleApi.update(editing.id, payload)
      message.success(t('ruleManager.updated'))
    } else {
      await ruleApi.create(payload)
      message.success(t('ruleManager.created'))
    }
    setOpen(false)
    load()
  }

  const onDelete = async (id: string) => {
    await ruleApi.delete(id)
    message.success(t('ruleManager.deleted'))
    load()
  }

  return (
    <div className="page-container">
      <Card
        title={<Title level={4} style={{ margin: 0 }}>{title}</Title>}
        extra={
          <Space>
            {withProject && !lockedProjectId && (
              <Select
                placeholder={t('ruleManager.selectProject')}
                style={{ width: 220 }}
                value={projectId}
                onChange={setProjectId}
                options={projects.map((p) => ({ value: p.id, label: `${p.name} (${p.slug})` }))}
              />
            )}
            <Button type="primary" icon={<PlusOutlined />} onClick={onNew}>
              {t('ruleManager.newRule')}
            </Button>
          </Space>
        }
      >
        <Text type="secondary" style={{ display: 'block', marginBottom: 12 }}>
          {description}
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
            { title: t('ruleManager.colName'), dataIndex: 'name' },
            {
              title: t('ruleManager.colType'),
              dataIndex: 'type',
              width: 160,
              render: (v) => <Tag color="purple">{v}</Tag>
            },
            {
              title: t('ruleManager.colScope'),
              dataIndex: 'scope',
              width: 100,
              render: (v) => <Tag>{v}</Tag>
            },
            {
              title: 'Agent',
              dataIndex: 'agent_name',
              width: 120,
              render: (v) => (v ? <Tag color="blue">{v}</Tag> : <Text type="secondary">all</Text>)
            },
            {
              title: t('ruleManager.colDescription'),
              dataIndex: 'description',
              ellipsis: true
            },
            {
              title: t('ruleManager.colVersion'),
              dataIndex: 'version',
              width: 60
            },
            {
              title: t('ruleManager.colUpdated'),
              dataIndex: 'updated_at',
              width: 170,
              render: (v) => formatDate(v)
            },
            {
              title: t('ruleManager.colActions'),
              width: 180,
              render: (_, r) => (
                <Space>
                  <Button type="link" icon={<EditOutlined />} onClick={() => onEdit(r)}>
                    {t('ruleManager.edit')}
                  </Button>
                  <Popconfirm title={t('ruleManager.confirmDelete')} onConfirm={() => onDelete(r.id)}>
                    <Button type="link" danger icon={<DeleteOutlined />}>
                      {t('ruleManager.delete')}
                    </Button>
                  </Popconfirm>
                </Space>
              )
            }
          ]}
        />
      </Card>

      <Modal
        title={editing ? t('ruleManager.modalEdit') : t('ruleManager.modalNew')}
        open={open}
        onCancel={() => setOpen(false)}
        onOk={onSubmit}
        destroyOnClose
        width={720}
      >
        <Form form={form} layout="vertical">
          <Form.Item name="name" label={t('ruleManager.labelName')} rules={[{ required: true }]}>
            <Input placeholder={t('ruleManager.placeholderName')} />
          </Form.Item>
          {!fixedType && (
            <Form.Item name="type" label={t('ruleManager.labelType')} rules={[{ required: true }]}>
              <Select options={resolvedTypeOptions} />
            </Form.Item>
          )}
          <Form.Item name="description" label={t('ruleManager.labelDescription')}>
            <Input placeholder={t('ruleManager.placeholderDescription')} />
          </Form.Item>
          <Form.Item name="value" label={t('ruleManager.labelValue')} rules={[{ required: true }]}>
            <TextArea
              rows={6}
              placeholder={t('ruleManager.placeholderValue')}
            />
          </Form.Item>
          <Form.Item name="agent_name" label={t('ruleManager.labelAgent')}>
            <Input placeholder="cursor / claude_code / windsurf / vscode" />
          </Form.Item>
          <Form.Item name="tags" label={t('ruleManager.labelTags')}>
            <Input placeholder="security, code_quality" />
          </Form.Item>
          <Form.Item name="version" label={t('ruleManager.labelVersion')} initialValue={1}>
            <InputNumber min={1} style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export default RuleManager
