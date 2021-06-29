<template>
  <div id="block">
    <a-row :gutter="64">
      <a-col :span="8">
        <a-statistic-countdown
          title="区块高度"
          :value="deadline"
          style="margin-right: 50px"
          @finish="onFinish"
        />
      </a-col>
      <a-col :span="8">
        <a-statistic-countdown
          title="累计交易数量"
          :value="deadline"
          format="HH:mm:ss:SSS"
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
export default {
  name: "blockStatue",
  data() {
    return {
      deadline: Date.now() + 1000 * 60 * 60 * 24 * 2 + 1000 * 30,
    };
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