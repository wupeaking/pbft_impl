<template>
  <div id="block" style="margin-top:10px">
    <a-row :gutter="64">
      <a-col :span="6">
        <a-statistic
          title="区块高度"
          :value="blockNumber"
          style="margin-right: 50px"
          @finish="onFinish"
        />
      </a-col>
      <a-col :span="6">
        <a-statistic
          title="累计交易数量"
          :value="txNumber"
          style="margin-right: 50px"
        />
      </a-col>
      <a-col :span="6">
        <a-statistic
          title="验证节点"
          :value="verfiers"
          style="margin-right: 50px"
        />
      </a-col>
      <a-col :span="6">
        <a-statistic
          title="视图编号"
          :value="view"
        />
      </a-col>
    </a-row>
  </div>
</template>
<script>
import axios from "axios";
export default {
  name: "blockStatue",
  data() {
    return {
      view: 0,
      blockNumber: 0,
      verfiers: 0,
      txNumber: 0,
    };
  },
  mounted() {
    setInterval(() => {
      this.deadline++;
      var that = this;
      axios
        .get("/api/ws/status")
        .then(function(response) {
          // console.log(response)
          if(response.data.code !== 0) {
            console.log(response.msg)
          }else{
            // console.log(response)
            that.blockNumber = response.data.data.block_num;
            that.txNumber = response.data.data.tx_num;
            that.verfiers = response.data.data.verfier_num;
            that.view = response.data.data.last_view;
          }
        })
        .catch((error) => console.log(error));
    }, 2000);
  },
  methods: {
    onFinish() {
      console.log("finished!");
    },
  },
};
</script>

<style scoped>
#blockxx {
  height: 300px;
}
</style>