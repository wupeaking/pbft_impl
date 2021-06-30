<template>
  <div>
    <a-divider style="font-size:1px">最近交易</a-divider>
    <a-table :columns="columns" :data-source="data"> </a-table>
  </div>
</template>
<script>
var columns = [
  {
    title: "交易hash",
    dataIndex: "hash",
    key: "hash",
    // scopedSlots: { customRender: "hash" },
    ellipsis: true,
  },
  {
    title: "交易金额",
    dataIndex: "amount",
    key: "amount",
    scopedSlots: { customRender: "amount" },
  },
  {
    title: "发送方",
    dataIndex: "from",
    key: "from",
    ellipsis: true,
  },
  {
    title: "接收方",
    dataIndex: "to",
    key: "to",
    ellipsis: true,
  },
];

var data = [
  {
    hash: "221458a471f813c596fcd83248b080d219d007f7dc1ac8173106891514e11b75ecede060dd9f8140befc80c097db4d153bf9a016d1b7cb0",
    from: "0x8e1fbf5b13279c82eac11cc23f456118d12a1babdecd9dbfb643defe4a1d9e62",
    to: "0xf52772d71e21a42e8cd2c5987ed3bb99420fecf4c7aca797b926a8f01ea6ffd8",
    amount: "100",
  },
  {
    hash: "221458a471f813c596fcd83248b080d219d007f7dc1ac8173106891514e11b75ecede060dd9f8140befc80c097db4d153bf9a016d1b7cb0",
    from: "0x8e1fbf5b13279c82eac11cc23f456118d12a1babdecd9dbfb643defe4a1d9e62",
    to: "0xf52772d71e21a42e8cd2c5987ed3bb99420fecf4c7aca797b926a8f01ea6ffd8",
    amount: "100",
  },
];

import axios from "axios";

export default {
  name: "lastTx",
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
        .get("/api/ws/last_txs")
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
                hash: data.data[d].tx_id,
                from: data.data[d].from,
                to: data.data[d].to,
                amount: data.data[d].amount,
              });
            }
          }
        })
        .catch((error) => console.log(error));
    }, 2000);
  },
};
</script>