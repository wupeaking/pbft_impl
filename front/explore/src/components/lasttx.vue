<template>
  <div>
    <a-table :columns="columns" :data-source="data">
      
    </a-table>
  </div>
</template>
<script>
const columns = [
  {
    title: "交易编号",
    dataIndex: "name",
    key: "name",
    scopedSlots: { customRender: "name" },
  },
  {
    title: "交易hash",
    dataIndex: "hash",
    key: "hash",
    // width: 80,
  },
  {
    title: "交易收据",
    dataIndex: "tx_num",
    key: "tx_num",
    ellipsis: true,
  }
];

const data = [
  {
    name: "John Brown",
    tx_num: 32,
    hash: "New York No. 1 Lake Park, New York No. 1 Lake Park",
  },
  {
    name: "Jim Green",
    tx_num: 42,
    hash: "London No. 2 Lake Park, London No. 2 Lake Park",
  },
  {
    name: "Joe Black",
    tx_num: 32,
    hash: "Sidney No. 1 Lake Park, Sidney No. 1 Lake Park",
  },
    {
    name: "John Brown",
    tx_num: 32,
    hash: "New York No. 1 Lake Park, New York No. 1 Lake Park",
  },
  {
    name: "Jim Green",
    tx_num: 42,
    hash: "London No. 2 Lake Park, London No. 2 Lake Park",
  },
  {
    name: "Joe Black",
    tx_num: 32,
    hash: "Sidney No. 1 Lake Park, Sidney No. 1 Lake Park",
  },
];

export default {
  name: "lastTx",
  data() {
    return {
      data,
      columns,
    };
  },
};
</script>