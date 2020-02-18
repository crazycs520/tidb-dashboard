import React from 'react'
import { Table } from 'antd'
import { StatementDetailInfo } from './statement-types'

type align = 'left' | 'right' | 'center'

const columns = [
  {
    title: 'kind',
    dataIndex: 'kind',
    key: 'kind',
    align: 'center' as align,
    width: 160
  },
  {
    title: 'content',
    dataIndex: 'content',
    key: 'content',
    align: 'left' as align
  }
]

export default function StatementSummaryTable({
  detail
}: {
  detail: StatementDetailInfo
}) {
  const dataSource = [
    {
      kind: 'Schema',
      content: detail.schema_name
    },
    {
      kind: 'SQL 类别',
      content: detail.digest_text
    },
    {
      kind: '最后出现 SQL 语句',
      content: detail.query_sample_text
    },
    {
      kind: '最后出现时间',
      content: detail.last_seen
    }
  ]

  return (
    <Table
      columns={columns}
      dataSource={dataSource}
      rowKey="kind"
      pagination={false}
      showHeader={false}
    />
  )
}
