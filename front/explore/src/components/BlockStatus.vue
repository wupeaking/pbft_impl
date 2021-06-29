<template>
  <div id="block">
    <a-row :gutter="64">
      <a-col :span="8">
        <a-statistic
          title="区块高度"
          :value="deadline"
          style="margin-right: 50px"
          @finish="onFinish"
        />
      </a-col>
      <a-col :span="8">
        <a-statistic
          title="累计交易数量"
          :value="deadline"
          style="margin-right: 50px"
        />
      </a-col>
      <a-col :span="8">
        <a-statistic-countdown
          title="成功运行天数"
          :value="deadline"
          format="D 天 H 时 m 分 s 秒"
        />
      </a-col>
    </a-row>
  </div>
</template>
<script>
import axios from 'axios';
export default {
  name: "blockStatue",
  data() {
    return {
      deadline: 0,
    };
  },
  mounted() {
    setInterval(() => {
        console.log("get ----------------");
        this.deadline++;
      axios
        .get("https://api.coindesk.com/v1/bpi/currentprice.json")
        .then((response) => (this.info = response.data.bpi))
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