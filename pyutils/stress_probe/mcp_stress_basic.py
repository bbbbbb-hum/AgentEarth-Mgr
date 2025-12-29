import sys
import os
import json
import asyncio
from datetime import datetime
import requests
import logging
from mcp import ClientSession
from mcp.client.streamable_http import streamablehttp_client

_config_cache = None
_config_mtime = None
global_logger = None

def calculate_percentile(data, percentile):
    """计算百分位数
    
    Args:
        data: 数据列表
        percentile: 百分位数（0-100）
    
    Returns:
        float: 百分位数值
    """
    sorted_data = sorted(data)
    n = len(sorted_data)
    if n == 0:
        return 0
    k = (n - 1) * percentile / 100
    f = int(k)
    c = f + 1 if f + 1 < n else f
    if f == c:
        return sorted_data[f]
    return sorted_data[f] + (k - f) * (sorted_data[c] - sorted_data[f])

def setup_global_logger():
    """设置全局日志记录器"""
    global global_logger
    if global_logger is not None:
        return global_logger
    
    config = load_config()
    log_config = config['LogConfig'] if config else None
    if not log_config:
        raise ValueError("LogConfig配置项不存在")
    
    log_path = log_config['log_path']
    log_level_str = log_config['log_level'].upper()
    log_level = getattr(logging, log_level_str, logging.INFO)
    console_format = log_config['console_format']
    file_format = log_config['file_format']
    
    global_logger = logging.getLogger('MCPStressBasic')
    global_logger.setLevel(log_level)
    
    if not global_logger.handlers:
        console_handler = logging.StreamHandler(sys.stdout)
        file_handler = logging.FileHandler(log_path, mode='a', encoding='utf-8')
        console_formatter = logging.Formatter(console_format)
        file_formatter = logging.Formatter(file_format)
        console_handler.setFormatter(console_formatter)
        file_handler.setFormatter(file_formatter)
        global_logger.addHandler(console_handler)
        global_logger.addHandler(file_handler)
    
    global_logger.propagate = False
    
    return global_logger

def load_config(config_file='config.json'):
    """加载JSON配置文件（带缓存和变更检测）
    
    Returns:
        dict: 配置字典，如果加载失败则返回 None
    """
    global _config_cache, _config_mtime
    
    try:
        config_path = "/opt/xlconfigs/AEMgr/pyutils/" + config_file
        
        if not os.path.exists(config_path):
            logger = setup_global_logger()
            logger.error(f"配置文件不存在: {config_path}")
            return None
        
        current_mtime = os.path.getmtime(config_path)
        
        if _config_cache is not None and _config_mtime is not None:
            if current_mtime == _config_mtime:
                return _config_cache
        
        with open(config_path, 'r', encoding='utf-8') as f:
            config = json.load(f)
        
        _config_cache = config
        _config_mtime = current_mtime
        return config
    except json.JSONDecodeError as e:
        logger = setup_global_logger()
        logger.error(f"配置文件格式错误: {str(e)}")
        return None
    except Exception as e:
        logger = setup_global_logger()
        logger.error(f"加载配置文件失败: {str(e)}")
        return None

def get_enabled_services():
    """从API获取服务列表，使用display_name作为服务名称"""
    response = None
    logger = setup_global_logger()
    
    try:
        config = load_config()
        if not config:
            logger.error("配置加载失败，无法获取服务列表")
            return []
        
        service_list_api = config.get('ServiceListApi')
        if not service_list_api or 'url' not in service_list_api:
            logger.error("服务列表API配置不存在或不完整")
            return []
        
        api_url = service_list_api['url']
        logger.info(f"尝试从API获取服务列表: {api_url}")
        
        response = requests.get(api_url, timeout=30)
        
        if response.status_code != 200:
            logger.error(f"获取服务列表失败: HTTP状态码 {response.status_code}")
            logger.error(f"响应内容: {response.text}")
            return []
        
        data = response.json()
        
        services_data = []
        
        if isinstance(data, dict):
            if 'data' in data and isinstance(data['data'], dict) and 'list' in data['data'] and isinstance(data['data']['list'], list):
                services_data = data['data']['list']
        elif isinstance(data, list):
            services_data = data
        else:
            logger.error(f"服务列表API返回数据格式异常: 期望字典或列表类型，实际类型 {type(data).__name__}")
            logger.error(f"原始响应数据: {data}")
            return []
        
        services = []
        seen_server_ids = set()
        
        for item in services_data:
            if isinstance(item, dict) and 'server_id' in item and 'display_name' in item:
                server_id = item['server_id']
                if server_id not in seen_server_ids:
                    seen_server_ids.add(server_id)
                    services.append({
                        "server_id": server_id,
                        "server_name": item['display_name']
                    })
                else:
                    logger.warning(f"忽略重复服务: ID={server_id}, 名称={item['display_name']}")
        
        if services:
            logger.info(f"成功获取 {len(services)} 个服务")
        else:
            logger.warning("警告: 未能从API响应中提取到有效的服务信息")
        
        return services
        
    except requests.exceptions.RequestException as e:
        logger.error(f"获取服务列表网络错误: {str(e)}")
        return []
    except json.JSONDecodeError as e:
        logger.error(f"解析服务列表响应失败: {str(e)}")
        if response:
            logger.error(f"原始响应内容: {response.text[:200]}...")
        return []
    except Exception as e:
        logger.error(f"获取服务列表失败: {str(e)}")
        return []

def get_api_info():
    logger = setup_global_logger()
    config = load_config()
    if not config:
        logger.error("配置加载失败，无法获取API信息")
        return None
    
    api_info = config['ApiInfo']
    if not api_info:
        logger.error("API配置信息不存在")
        return None
    
    api_info = api_info.copy()
    
    if 'X-API-Key' not in api_info or not api_info['X-API-Key']:
        logger.error("错误: API密钥不能为空")
        return None
    
    logger.info(f"API配置已加载: addr={api_info['addr']}")
    return api_info

class MCPLevel1Prober:
    def __init__(self, concurrent_users=None, requests_per_user=None):
        self.services = []
        self.config = None
        self.api_info = None
        self.probe_results = []
        self.concurrent_users = concurrent_users
        self.requests_per_user = requests_per_user
        self.request_timeout = None
        self.probe_completed_requests = 0
        self.probe_total_requests = 0
        self.probe_test_start_time = None
        self.probe_successful_requests = 0
        self._load_probe_config()
    
    def _load_probe_config(self):
        """加载探测配置"""
        logger = setup_global_logger()
        config = load_config()
        if not config:
            logger.error("错误: 配置加载失败")
            sys.exit(1)
        
        probe_config = config['ProbeConfig']
        
        if self.concurrent_users is None:
            self.concurrent_users = probe_config['concurrent_users']
        if self.requests_per_user is None:
            self.requests_per_user = probe_config['requests_per_user']
        self.request_timeout = probe_config['request_timeout']
        
        if not self.concurrent_users:
            logger.error("错误: 配置文件中缺少 concurrent_users 参数")
            sys.exit(1)
        if not self.requests_per_user:
            logger.error("错误: 配置文件中缺少 requests_per_user 参数")
            sys.exit(1)
        if not self.request_timeout:
            logger.error("错误: 配置文件中缺少 request_timeout 参数")
            sys.exit(1)
        
        logger.info(f"探测配置已加载: 并发用户数={self.concurrent_users}, 每用户请求数={self.requests_per_user}, 请求超时={self.request_timeout}秒")

    def initialize(self):
        logger = setup_global_logger()
        logger.info("开始初始化MCP一级探测器...")
        
        logger.info("步骤1: 加载基础配置")
        self.config = load_config()
        if not self.config:
            logger.error("错误: 无法加载基础配置")
            return False
        logger.info("基础配置加载成功")
        
        logger.info("步骤2: 获取API信息")
        self.api_info = get_api_info()
        if not self.api_info:
            logger.error("错误: 无法获取API信息")
            return False
        
        if not all(k in self.api_info for k in ['addr', 'X-API-Key']):
            logger.error("错误: API信息不完整，缺少必要字段")
            return False
        logger.info(f"API信息获取成功，基础URL: {self.api_info['addr']}")
        
        logger.info("步骤3: 从API获取服务列表")
        self.services = get_enabled_services()
        if not self.services:
            logger.warning("警告: 未获取到任何服务")
        
        logger.info("MCP一级探测器初始化完成")
        return True
    
    def _create_probe_result(self, user_id, probe_index, server_name, server_id, status, tools_count=0, error='', probe_time=0):
        """创建探测结果字典
        
        Args:
            user_id: 用户ID
            probe_index: 探测索引
            server_name: 服务名称
            server_id: 服务ID
            status: 探测状态
            tools_count: 工具数量
            error: 错误信息
            probe_time: 探测耗时
        
        Returns:
            dict: 探测结果字典
        """
        return {
            'user_id': user_id,
            'probe_index': probe_index,
            'server_name': server_name,
            'server_id': server_id,
            'timestamp': datetime.now(),
            'status': status,
            'tools_count': tools_count,
            'error': error,
            'probe_time': probe_time
        }
    
    def _handle_probe_error(self, user_id, probe_index, server_name, server_id, status, error, probe_start_time):
        """处理探测错误
        
        Args:
            user_id: 用户ID
            probe_index: 探测索引
            server_name: 服务名称
            server_id: 服务ID
            status: 探测状态
            error: 错误信息
            probe_start_time: 探测开始时间
        """
        probe_time = datetime.now().timestamp() - probe_start_time
        probe_result = self._create_probe_result(
            user_id, probe_index + 1, server_name, server_id,
            status=status, error=error, probe_time=probe_time
        )
        self.probe_results.append(probe_result)
        self.probe_completed_requests += 1
    
    async def basic_probe_worker(self, user_id, services):
        logger = setup_global_logger()
        
        base_url = self.api_info['addr']
        if not base_url:
            logger.error(f"错误: 无法获取基础URL，跳过用户 {user_id} 的探查")
            return
        
        if not services:
            logger.error(f"错误: 服务列表为空，跳过用户 {user_id} 的探查")
            return
        
        headers = {
            'X-API-Key': self.api_info['X-API-Key']
        }
        
        for probe_index in range(self.requests_per_user):
            service_index = (user_id * self.requests_per_user + probe_index) % len(services)
            service = services[service_index]
            server_name = service['server_name']
            server_id = service['server_id']
            
            server_url = f"{base_url}/mcp-server/{server_id}"
            
            probe_start_time = datetime.now().timestamp()
            
            try:
                async with streamablehttp_client(server_url, headers=headers) as (read, write, _):
                    async with ClientSession(read, write) as session:
                        await session.initialize()
                        tools_result = await session.list_tools()
                        probe_time = datetime.now().timestamp() - probe_start_time
                        
                        if hasattr(tools_result, 'tools') and tools_result.tools:
                            probe_result = self._create_probe_result(
                                user_id, probe_index + 1, server_name, server_id,
                                status='success', tools_count=len(tools_result.tools), probe_time=probe_time
                            )
                            self.probe_successful_requests += 1
                        else:
                            probe_result = self._create_probe_result(
                                user_id, probe_index + 1, server_name, server_id,
                                status='failed', error='无法获取工具定义', probe_time=probe_time
                            )
                        
                        self.probe_results.append(probe_result)
                        self.probe_completed_requests += 1
                        
            except asyncio.TimeoutError:
                self._handle_probe_error(user_id, probe_index, server_name, server_id, 'timeout', '连接超时', probe_start_time)
                
            except ConnectionError as e:
                self._handle_probe_error(user_id, probe_index, server_name, server_id, 'connection_error', f'连接失败: {str(e)}', probe_start_time)
                
            except Exception as e:
                self._handle_probe_error(user_id, probe_index, server_name, server_id, 'error', str(e), probe_start_time)
    
    async def probe_progress_monitor(self):
        logger = setup_global_logger()
        while True:
            if self.probe_completed_requests > 0:
                progress = (self.probe_completed_requests / self.probe_total_requests) * 100
                
                elapsed_time = (datetime.now() - self.probe_test_start_time).total_seconds()
                elapsed_time_str = f"{elapsed_time:.1f}秒"
                
                avg_time_per_request = elapsed_time / self.probe_completed_requests
                remaining_requests = self.probe_total_requests - self.probe_completed_requests
                eta_seconds = remaining_requests * avg_time_per_request
                eta = f"{eta_seconds:.1f}秒"
                
                success_rate = (self.probe_successful_requests / self.probe_completed_requests * 100) if self.probe_completed_requests > 0 else 0
                
                logger.info(f"压力测试进度: {self.probe_completed_requests}/{self.probe_total_requests} ({progress:.1f}%) | "
                      f"成功: {self.probe_successful_requests} ({success_rate:.1f}%) | "
                      f"已用时间: {elapsed_time_str} | "
                      f"预计剩余时间: {eta}")
            
            await asyncio.sleep(1)
    
    async def basic_probe_all_services(self):
        logger = setup_global_logger()
        
        if not self.services:
            logger.error("错误: 没有可用的服务进行探查")
            return
        
        logger.info(f"正在执行压力测试: {self.concurrent_users}个并发用户，每用户{self.requests_per_user}次请求")
        
        self.probe_results = []
        
        self.probe_total_requests = self.concurrent_users * self.requests_per_user
        self.probe_test_start_time = datetime.now()
        self.probe_completed_requests = 0
        self.probe_successful_requests = 0
        
        progress_task = asyncio.create_task(self.probe_progress_monitor())
        
        tasks = []
        for user_id in range(self.concurrent_users):
            task = asyncio.create_task(
                self.basic_probe_worker(user_id, self.services)
            )
            tasks.append(task)
        
        await asyncio.gather(*tasks)
        
        progress_task.cancel()
        try:
            await progress_task
        except asyncio.CancelledError:
            pass
        
        logger.info(f"压力测试完成")
        self.print_probe_summary()
    
    def _calculate_time_statistics(self, probe_times):
        """计算时延统计
        
        Args:
            probe_times: 时延列表
        
        Returns:
            dict: 包含平均、最小、最大、p75、p95、p99的字典
        """
        if not probe_times:
            return {
                'avg': 0,
                'min': 0,
                'max': 0,
                'p75': 0,
                'p95': 0,
                'p99': 0
            }
        
        return {
            'avg': sum(probe_times) / len(probe_times),
            'min': min(probe_times),
            'max': max(probe_times),
            'p75': calculate_percentile(probe_times, 75),
            'p95': calculate_percentile(probe_times, 95),
            'p99': calculate_percentile(probe_times, 99)
        }
    
    def _build_service_stats(self):
        """构建服务统计信息
        
        Returns:
            tuple: (service_stats, total_probes, total_success, all_probe_times)
        """
        service_stats = {}
        for result in self.probe_results:
            server_name = result['server_name']
            if server_name not in service_stats:
                service_stats[server_name] = {
                    'total': 0,
                    'success': 0,
                    'failed': 0,
                    'error': 0,
                    'timeout': 0,
                    'probe_times': []
                }
            
            service_stats[server_name]['total'] += 1
            service_stats[server_name][result['status']] += 1
            
            if result.get('status') == 'success' and result.get('probe_time') is not None:
                service_stats[server_name]['probe_times'].append(result['probe_time'])
        
        total_probes = len(self.probe_results)
        total_success = sum(1 for r in self.probe_results if r['status'] == 'success')
        all_probe_times = [r['probe_time'] for r in self.probe_results if r['status'] == 'success' and r['probe_time'] is not None]
        
        return service_stats, total_probes, total_success, all_probe_times
    
    def print_probe_summary(self):
        """打印压力测试结果摘要"""
        if not self.probe_results:
            logger.info("没有探测结果")
            return
        
        service_stats, total_probes, total_success, all_probe_times = self._build_service_stats()
        success_rate = (total_success / total_probes * 100) if total_probes > 0 else 0
        
        time_stats = self._calculate_time_statistics(all_probe_times)
        
        slow_services = []
        for server_name, stats in service_stats.items():
            if stats['probe_times']:
                service_avg_time = sum(stats['probe_times']) / len(stats['probe_times'])
                if service_avg_time > 2.0:
                    slow_services.append({
                        'server_name': server_name,
                        'avg_time': service_avg_time,
                        'total': stats['total'],
                        'success': stats['success']
                    })
        
        logger.info("测试完成统计:")
        logger.info(f"总探测次数: {total_probes}次，成功探测: {total_success}次，失败探测{total_probes - total_success}次，成功率{success_rate:.1f}%")
        
        if all_probe_times:
            logger.info("探测时延统计（仅成功请求）:")
            logger.info(f"平均探测时延: {time_stats['avg']:.3f}秒")
            logger.info(f"最小探测时延: {time_stats['min']:.3f}秒")
            logger.info(f"最大探测时延: {time_stats['max']:.3f}秒")
            logger.info(f"p75探测时延: {time_stats['p75']:.3f}秒")
            logger.info(f"p95探测时延: {time_stats['p95']:.3f}秒")
            logger.info(f"p99探测时延: {time_stats['p99']:.3f}秒")
        
        if slow_services:
            logger.info(f"瓶颈服务（平均时延 > 2秒）: {len(slow_services)}个")
            for service in slow_services:
                logger.info(f"{service['server_name']}: 平均时延{service['avg_time']:.3f}秒，总请求{service['total']}次，成功{service['success']}次")
        else:
            logger.info("没有瓶颈服务（平均时延 > 2秒）")
    
    async def run(self):
        logger = setup_global_logger()
        logger.info("="*60)
        logger.info("MCP压力测试工具 v1.0")
        logger.info("功能: 连接服务并获取工具定义")
        logger.info("="*60)
        
        if not self.initialize():
            logger.error("初始化失败，无法继续执行探测")
            return False
        
        await self.basic_probe_all_services()
        
        return True

if __name__ == "__main__":
    import argparse
    
    parser = argparse.ArgumentParser(description='MCP压力测试工具')
    parser.add_argument('--concurrent-users', type=int, help='并发用户数（默认: 50）')
    parser.add_argument('--requests-per-user', type=int, help='每个用户请求数（默认: 20）')
    args = parser.parse_args()
    
    logger = setup_global_logger()
    logger.info("="*60)
    logger.info("正在启动压力测试...")
    prober = MCPLevel1Prober(
        concurrent_users=args.concurrent_users,
        requests_per_user=args.requests_per_user
    )
    
    try:
        result = asyncio.run(prober.run())
    except KeyboardInterrupt:
        logger.warning("用户中断程序，正在停止探测...")
        logger.warning("程序已退出")
        sys.exit(0)
    
    if result:
        logger.info("压力测试完成!")
        sys.exit(0)
    else:
        logger.error("压力测试失败!")
        sys.exit(1)
