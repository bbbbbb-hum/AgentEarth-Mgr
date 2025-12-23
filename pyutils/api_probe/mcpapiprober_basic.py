import sys
import os
import json
import asyncio
from datetime import datetime
import traceback
import requests
import logging

api_probe_dir = os.path.dirname(os.path.abspath(__file__))
sys.path.append(api_probe_dir)

def load_config():
    """从配置文件加载API配置
    
    Returns:
        dict: 配置字典，如果加载失败则返回 None
    """
    config_file = "config.json"
    config_path = "/opt/xlconfigs/AEMgr/pyutils/api_probe/" + config_file
    
    try:
        with open(config_path, 'r', encoding='utf-8') as f:
            config = json.load(f)
        return config
    except Exception as e:
        print(f"加载配置文件失败: {str(e)}")
        return None

def get_enabled_services():
    """从API获取服务列表，使用display_name作为服务名称"""
    response = None
    try:
        config = load_config()
        if not config:
            print("配置加载失败，无法获取服务列表")
            return []
        
        # 获取服务列表API配置
        service_list_api = config.get('ServiceListApi')
        if not service_list_api or 'url' not in service_list_api:
            print("服务列表API配置不存在或不完整")
            return []
        
        api_url = service_list_api['url']
        print(f"尝试从API获取服务列表: {api_url}")
        
        # 发送GET请求获取服务列表
        response = requests.get(api_url, timeout=30)
        
        # 检查响应状态
        if response.status_code != 200:
            print(f"获取服务列表失败: HTTP状态码 {response.status_code}")
            print(f"响应内容: {response.text}")
            return []
        
        # 解析JSON响应
        data = response.json()
        
        # 提取服务信息列表
        services_data = []
        
        # 处理不同的数据格式
        if isinstance(data, dict):
            # 检查数据格式：data.list
            if 'data' in data and isinstance(data['data'], dict) and 'list' in data['data'] and isinstance(data['data']['list'], list):
                services_data = data['data']['list']
        elif isinstance(data, list):
            # 直接使用列表格式
            services_data = data
        else:
            print(f"服务列表API返回数据格式异常: 期望字典或列表类型，实际类型 {type(data).__name__}")
            print(f"原始响应数据: {data}")
            return []
        
        # 提取服务信息并根据server_id去重
        services = []
        seen_server_ids = set()  # 用于存储已见过的server_id
        
        for item in services_data:
            if isinstance(item, dict) and 'server_id' in item and 'display_name' in item:
                server_id = item['server_id']
                # 如果server_id未见过，则添加到结果列表并标记为已见
                if server_id not in seen_server_ids:
                    seen_server_ids.add(server_id)
                    services.append({
                        "server_id": server_id,
                        "server_name": item['display_name']  # 使用display_name作为服务名称
                    })
                else:
                    print(f"忽略重复服务: ID={server_id}, 名称={item['display_name']}")
        
        if services:
            print(f"成功获取 {len(services)} 个服务")
        else:
            print("警告: 未能从API响应中提取到有效的服务信息")
        
        return services
        
    except requests.exceptions.RequestException as e:
        print(f"获取服务列表网络错误: {str(e)}")
        print(traceback.format_exc())
        return []
    except json.JSONDecodeError as e:
        print(f"解析服务列表响应失败: {str(e)}")
        print(f"原始响应内容: {response.text[:200]}...")
        print(traceback.format_exc())
        return []
    except Exception as e:
        print(f"获取服务列表失败: {str(e)}")
        print(traceback.format_exc())
        return []

def get_api_info():
    """获取API信息"""
    config = load_config()
    if not config:
        print("配置加载失败，无法获取API信息")
        return None
    
    api_info = config.get('ApiInfo', None)
    if not api_info:
        print("API配置信息不存在")
        return None
    
    if 'X-API-Key' not in api_info or not api_info['X-API-Key']:
        print("错误: API密钥不能为空")
        return None
    
    # 确保repeat_time_sec为整数且大于0
    if 'repeat_time_sec' not in api_info or not isinstance(api_info['repeat_time_sec'], int) or api_info['repeat_time_sec'] <= 0:
        print("警告: API repeat_time_sec设置无效，请在配置文件中正确设置repeat_time_sec值")
    
    print(f"API配置已加载: addr={api_info['addr']}, repeat_time_sec={api_info['repeat_time_sec']}秒")
    return api_info

from mcp import ClientSession
from mcp.client.streamable_http import streamablehttp_client

class MCPBasicProber:
    
    def __init__(self):
        """初始化MCP一级探查器，设置初始状态"""
        self.services = []
        self.config = None
        self.api_info = None
        self.probe_results = []
        self.probe_time = None
        self.cycle_count = 0
        
        # 设置日志记录器
        self.setup_logger()
    
    def _handle_error(self, server_name, error):
        """统一的错误处理函数
        
        Args:
            server_name (str): 服务器名称
            error (Exception): 异常对象
            
        Returns:
            dict: 错误结果字典
        """
        error_type = type(error).__name__
        error_msg = str(error)
        
        if isinstance(error, asyncio.TimeoutError):
            print(f"连接服务 {server_name} 超时")
            error_desc = "连接超时"
        elif isinstance(error, ConnectionError):
            print(f"连接服务 {server_name} 失败: {error_msg}")
            error_desc = f'连接错误: {error_msg}'
        else:
            print(f"获取服务 {server_name} 工具定义时发生异常: {error_type} - {error_msg}")
            error_desc = f'未知错误: {error_type} - {error_msg}'
        
        return {
            'server': server_name,
            'tool': 'tools_list',
            'status': 'error',
            'error': error_desc,
            'timestamp': datetime.now().strftime('%Y-%m-%d %H:%M:%S'),
            'execution_time': 0,
            'has_valid_result': False
        }
    
    def setup_logger(self):
        """设置日志记录器，同时输出到控制台和文件"""
        # 加载配置以获取日志设置
        config = load_config()
        log_config = config.get('LogConfig', {}) if config else {}
        
        # 获取日志文件路径
        log_file_path = log_config.get('log_file_path')
        
        print(f"日志文件路径: {log_file_path}")
        
        # 获取日志级别
        log_level_str = log_config.get('log_level', 'INFO').upper()
        log_level = getattr(logging, log_level_str, logging.INFO)
        
        # 获取日志格式
        console_format = log_config.get('console_format', '%(message)s')
        file_format = log_config.get('file_format', '%(asctime)s - %(levelname)s - %(message)s')
        
        # 创建日志记录器
        self.logger = logging.getLogger('MCPBasicProber')
        self.logger.setLevel(log_level)
        
        # 避免重复添加处理器
        if self.logger.handlers:
            return
        
        # 创建控制台处理器
        console_handler = logging.StreamHandler(sys.stdout)
        
        # 创建文件处理器
        file_handler = logging.FileHandler(log_file_path, mode='a', encoding='utf-8')
        
        # 创建格式器
        console_formatter = logging.Formatter(console_format)
        file_formatter = logging.Formatter(file_format)
        console_handler.setFormatter(console_formatter)
        file_handler.setFormatter(file_formatter)
        
        # 添加处理器到日志记录器
        self.logger.addHandler(console_handler)
        self.logger.addHandler(file_handler)
    
    def initialize(self):
        """初始化探查器：加载配置、获取服务列表"""
        print("开始初始化MCP接口一级探查器...")
        
        # 步骤1: 加载配置
        print("步骤1: 加载基础配置")
        self.config = load_config()
        if not self.config:
            print("错误: 无法加载基础配置")
            return False
        print("基础配置加载成功")
        
        # 步骤2: 获取API信息
        print("步骤2: 获取API信息")
        self.api_info = get_api_info()
        if not self.api_info:
            print("错误: 无法获取API信息")
            return False
        
        # 验证API信息的关键字段
        if not all(k in self.api_info for k in ['addr', 'X-API-Key']):
            print("错误: API信息不完整，缺少必要字段")
            return False
        print(f"API信息获取成功，基础URL: {self.api_info['addr']}")
        
        # 步骤3: 从API获取服务列表
        print("步骤3: 从API获取服务列表")
        self.services = get_enabled_services()
        if not self.services:
            print("警告: 未获取到任何服务")
        
        print("MCP接口一级探查器初始化完成")
        return True
    
    async def _get_tools_only(self, server_name, server_url, headers):
        """只获取工具定义，不调用工具
        Args:
            server_name (str): 服务器名称
            server_url (str): 服务器URL
            headers (dict): HTTP请求头
        """
        print(f"正在获取服务 {server_name} 的工具定义...")
        
        try:
            async with streamablehttp_client(server_url, headers=headers) as (read, write, _):
                async with ClientSession(read, write) as session:
                    await session.initialize()
                    
                    # 获取所有工具定义
                    tools_result = await session.list_tools()
                    
                    if hasattr(tools_result, 'tools') and tools_result.tools:
                        print(f"成功获取 {len(tools_result.tools)} 个工具定义")
                        
                        # 添加到结果列表
                        self.probe_results.append({
                            'server': server_name,
                            'tool': 'tools_list',
                            'status': 'success',
                            'error': None,
                            'timestamp': datetime.now().strftime('%Y-%m-%d %H:%M:%S'),
                            'execution_time': 0,
                            'has_valid_result': True,
                            'payload': {
                                'tools_count': len(tools_result.tools),
                                'tools': [
                                    {
                                        'name': tool.name,
                                        'description': tool.description if hasattr(tool, 'description') else '',
                                        'parameters': tool.schema if hasattr(tool, 'schema') else {}
                                    }
                                    for tool in tools_result.tools
                                ]
                            }
                        })
                    else:
                        print(f"无法获取服务 {server_name} 的工具定义")
                        self.probe_results.append({
                            'server': server_name,
                            'tool': 'tools_list',
                            'status': 'error',
                            'error': '无法获取工具定义列表',
                            'timestamp': datetime.now().strftime('%Y-%m-%d %H:%M:%S'),
                            'execution_time': 0,
                            'has_valid_result': False
                        })
                        
        except (asyncio.TimeoutError, ConnectionError, Exception) as e:
            self.probe_results.append(self._handle_error(server_name, e))
    
    async def probe_service_tools(self, service):
        """探查单个服务的工具定义
        
        Args:
            service (dict): 服务信息字典，包含server_id和server_name
        """
        # 获取服务基本信息
        server_id = service.get('server_id')
        server_name = service.get('server_name')
        
        # 验证必要的服务信息
        if not server_id or not server_name:
            print(f"警告: 服务信息不完整，跳过探查")
            return
            
        print(f"\n开始一级探测服务: {server_name} (ID: {server_id})")
        
        # 构建服务URL和头部信息
        base_url = self.api_info.get('addr', '')
        if not base_url:
            print(f"错误: 无法获取基础URL，跳过服务 {server_name} 的探查")
            return
            
        # 根据配置文件修正URL格式
        server_url = f"{base_url}/mcp-server/{server_id}"
        headers = {
            'X-API-Key': self.api_info.get('X-API-Key', ''),
            'Content-Type': 'application/json'
        }
        
        # 获取工具定义
        await self._get_tools_only(server_name, server_url, headers)
    
    async def run_probe_cycle(self):
        """运行一次完整的一级探测周期"""
        print("\n" + "="*60)
        print("开始执行MCP接口一级探测周期")
        print(f"开始时间: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
        print("="*60)
        
        # 初始化探测状态
        self.probe_time = datetime.now()
        self.probe_results = []
        
        # 检查是否有服务需要探测
        if not self.services:
            print("错误: 没有可用的服务进行探测")
            self.print_probe_summary()
            return
        
        print(f"准备探测 {len(self.services)} 个服务")
        
        # 探测每个服务
        for idx, service in enumerate(self.services, 1):
            print(f"\n\n[{idx}/{len(self.services)}] 开始探测服务")
            await self.probe_service_tools(service)
        
        print("\n" + "="*60)
        print(f"探测周期结束")
        print(f"结束时间: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
        print("="*60)
        
        # 打印探测结果摘要
        self.print_probe_summary()
    
    def print_probe_summary(self):
        """打印一级探测结果摘要"""
        print("\n" + "="*60)
        print("一级探测结果摘要")
        print("="*60)
        
        timestamp = datetime.now().strftime('%Y-%m-%d %H:%M:%S.%f')[:-3]
        
        # 如果没有探测结果，直接返回
        if not self.probe_results:
            print(f"{timestamp} - INFO - 没有探测结果")
            return
            
        # 按服务分组结果
        service_results = {}
        for result in self.probe_results:
            server_name = result['server']
            if server_name not in service_results:
                service_results[server_name] = {
                    'status': 'unknown',
                    'tools': [],
                    'error': None
                }
            
            if result['tool'] == 'tools_list' and result['status'] == 'success':
                service_results[server_name]['status'] = 'success'
                if 'payload' in result and 'tools' in result['payload']:
                    service_results[server_name]['tools'] = result['payload']['tools']
            elif result['tool'] == 'tools_list' and result['status'] == 'error':
                service_results[server_name]['status'] = 'error'
                service_results[server_name]['error'] = result['error']
        
        # 统计结果
        total_services = len(service_results)
        successful_services = sum(1 for r in service_results.values() if r['status'] == 'success')
        failed_services = total_services - successful_services
        total_tools = sum(len(r['tools']) for r in service_results.values())
        
        # 计算成功率（避免除零错误）
        success_rate = (successful_services / total_services * 100) if total_services > 0 else 0
        
        # 周期统计
        print(f"周期 {self.cycle_count} 测试完成统计:")
        print(f"{timestamp} - INFO - 总服务数: {total_services}个，连接成功: {successful_services}个，连接失败{failed_services}个，服务连接成功率{success_rate:.1f}%")
        print(f"{timestamp} - INFO - 总工具数: {total_tools}个")
        
        # 成功的服务及其工具数
        if successful_services > 0:
            print(f"\n{timestamp} - INFO - 连接成功的服务:")
            for server_name, result in service_results.items():
                if result['status'] == 'success':
                    print(f"  - {server_name}: {len(result['tools'])} 个工具")
        
        # 失败的服务详情
        if failed_services > 0:
            print(f"\n{timestamp} - INFO - 连接失败的服务:")
            for server_name, result in service_results.items():
                if result['status'] == 'error':
                    print(f"  - {server_name}: {result.get('error', '未知错误')}")
    

    async def main(self):
        """主函数 - 执行一级探测器的初始化，并周期性地运行探测周期"""
        print("\n" + "="*60)
        print("MCP接口一级探测工具 v1.0")
        print("功能: 从数据库读取服务接口，连接服务并获取工具定义")
        print("="*60)
        
        if not self.initialize():
            print("初始化失败，无法继续执行探测")
            return False
        
        print("初始化成功，准备开始探测")
        
        # 获取并验证循环时间间隔
        repeat_time_sec = self.api_info.get('repeat_time_sec', 60)
        if not isinstance(repeat_time_sec, int) or repeat_time_sec <= 0:
            print(f"警告: 配置的循环时间间隔 {repeat_time_sec} 无效，使用默认值60秒")
            repeat_time_sec = 60
        
        print(f"开始周期性一级探测，循环间隔: {repeat_time_sec}秒")
        print("按 Ctrl+C 退出程序")
        
        try:
            while True:
                self.cycle_count += 1
                print(f"\n{'='*60}")
                print(f"开始执行一级探测周期 #{self.cycle_count}")
                print(f"{'='*60}")
                
                # 运行探测周期
                await self.run_probe_cycle()
                
                print(f"\n一级探测周期 #{self.cycle_count} 完成，等待 {repeat_time_sec} 秒后开始下一个周期...")
                await asyncio.sleep(repeat_time_sec)
                
        except KeyboardInterrupt:
            print(f"\n用户中断，程序退出。共执行了 {self.cycle_count} 个一级探测周期。")
            raise  # 重新抛出KeyboardInterrupt，让外部捕获

# 主程序入口
if __name__ == "__main__":
    print("\n正在启动...")
    print("探测所有可用服务")
    
    prober = MCPBasicProber()
    
    try:
        result = asyncio.run(prober.main())
        if result:
            print("\n一级探测任务完成")
        else:
            print("\n一级探测任务失败")
    except KeyboardInterrupt:
        print("\n用户中断，程序退出")
    except ImportError as e:
        print(f"\n错误: 缺少必要的模块 - {str(e)}")
    except ConnectionError as e:
        print(f"\n错误: 网络连接失败 - {str(e)}")
        print("请检查网络连接和API地址设置")
    except Exception as e:
        print(f"\n错误: 程序发生未预期的异常 - {str(e)}")
        print("详细错误信息:")
        traceback.print_exc()