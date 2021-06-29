<template>
  <div>
    <a-row>
      <a-col :span="3"> 贝壳区块浏览器 </a-col>
      <a-col :span="18">
        <div id="search">
          <a-input-search
            placeholder="输入区块数/交易hash/账户地址"
            enter-button="Search"
            size="large"
            @search="onSearch"
          />
        </div>
      </a-col>
      <a-col :span="3"> </a-col>
    </a-row>
  </div>
</template>

<script>
export default {
  name: "Header",
  props: {
    msg: String,
  },
  methods: {
    onSearch(value) {
      console.log(value);
    },
  },
};
</script>

<style>
#search {
  text-align: center;
  margin-top: 10px;
}
</style>