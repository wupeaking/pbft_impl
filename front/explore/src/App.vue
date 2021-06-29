<template>
  <div id="app">
    <a-layout id="components-layout-demo-basic">
      <a-layout-header>
        <Header></Header>
      </a-layout-header>
      <a-layout-content>
        <a-row>
          <a-col :span="24">
            <blockStatue></blockStatue>
          </a-col>
        </a-row>
         <a-divider />
        <a-row>
          <a-col :span="1"> </a-col>
          <a-col :span="10">
            <lastBlock></lastBlock>
          </a-col>
          <a-col :span="2"> </a-col>
          <a-col :span="10">
            <lastTx></lastTx>
          </a-col>
          <a-col :span="1"> </a-col>
        </a-row>
      </a-layout-content>
    </a-layout>
  </div>
</template>

<script>
import Header from "./components/header.vue";
import blockStatue from "./components/BlockStatus.vue";
import lastBlock from "./components/lastblock.vue";
import lastTx from "./components/lasttx.vue";

export default {
  name: "App",
  components: {
    Header,
    blockStatue,
    lastBlock,
    lastTx,
  },
};
</script>

<style>
#app {
  font-family: Avenir, Helvetica, Arial, sans-serif;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
  text-align: center;
  color: #2c3e50;
  margin-top: 1px;
}

#app .ant-layout-header,
#app .ant-layout-footer {
  background: #7dbcea;
  color: #fff;
}
</style>
