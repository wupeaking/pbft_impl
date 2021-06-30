<template>
  <div id="lastBlock">
     <a-divider style="font-size:1px">最近区块</a-divider>
    <a-table :columns="columns" :data-source="data"> </a-table>
  </div>
</template>
<script>

import axios from "axios";
const columns = [
  {
    title: "区块编号",
    dataIndex: "number",
    key: "number",
    scopedSlots: { customRender: "number" },
  },
  {
    title: "区块hash",
    dataIndex: "hash",
    key: "hash",
    ellipsis: true,
  },
  {
    title: "交易数量",
    dataIndex: "tx_num",
    key: "tx_num",
    ellipsis: true,
  },
];

const data = [
  {
    number: 1,
    tx_num: 32,
    hash: "f37ced5bc037ec2f51be02bda14b54975582380d22e9a29d18566d38e56d8b47",
  },
  {
    number: 2,
    tx_num: 42,
    hash: "7404e6f3b311dbde88e9fd3ff38e9f4d61a725f5bcdeb996d584f83cced1da2e",
  },
  {
    number: 3,
    tx_num: 32,
    hash: "8bdd124115c7d7ecc069d8153e93eeb6708310c9ca10322052884c7d36364175",
  },
];

export default {
  name: "lastBlock",
  data() {
    return {
      data,
      columns,
    };
  },

  mounted() {
    setInterval(() => {
      this.deadline++;
      that = this;
      var that = this;
      axios
        .get("/api/ws/last_blocks")
        .then(function (response) {
          if (response.data.code !== 0) {
            console.log(response.msg);
          } else {
            var data = response.data;
            if (data.data.length <= 0) {
              return;
            }
            that.data = [];
            for (var d in data.data) {
              // console.log(d)
              that.data.push({
                number: data.data[d].block_num,
                tx_num: data.data[d].tx_num,
                hash: data.data[d].id,
              });
            }
          }
        })
        .catch((error) => console.log(error));
    }, 2000);
  },
};
</script>

<style scoped>
#lastBlock .a-table {
  margin-left: 10px;
}
</style>