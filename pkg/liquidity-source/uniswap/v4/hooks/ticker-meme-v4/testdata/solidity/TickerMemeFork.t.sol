// SPDX-License-Identifier: MIT
pragma solidity ^0.8.30;

import {Test} from "forge-std/Test.sol";
import {IPoolManager} from "v4-core/interfaces/IPoolManager.sol";
import {IUnlockCallback} from "v4-core/interfaces/callback/IUnlockCallback.sol";
import {IHooks} from "v4-core/interfaces/IHooks.sol";
import {PoolKey} from "v4-core/types/PoolKey.sol";
import {PoolId} from "v4-core/types/PoolId.sol";
import {Currency} from "v4-core/types/Currency.sol";
import {BalanceDelta} from "v4-core/types/BalanceDelta.sol";
import {SwapParams, ModifyLiquidityParams} from "v4-core/types/PoolOperation.sol";
import {StateLibrary} from "v4-core/libraries/StateLibrary.sol";
import {FeePolicySnapshot} from "ticker/v2/interfaces/ILaunchpadV2.sol";

interface IMeme {
    function owner() external view returns (address);
    function setFactory(address) external;
    function registerPool(PoolKey calldata, address, address, address, uint16, bool, FeePolicySnapshot calldata)
        external;
    function pendingFees(PoolId, address) external view returns (uint256);
    function pendingCreatorTax(PoolId, address) external view returns (uint256);
    function pendingBuyback(PoolId, address) external view returns (uint256);
}

contract FixtureToken {
    mapping(address => uint256) public balanceOf;

    function mint(address to, uint256 amount) external {
        balanceOf[to] += amount;
    }

    function transfer(address to, uint256 amount) external returns (bool) {
        balanceOf[msg.sender] -= amount;
        balanceOf[to] += amount;
        return true;
    }
}

contract TickerMemeForkTest is Test, IUnlockCallback {
    using StateLibrary for IPoolManager;
    IPoolManager constant manager = IPoolManager(0x188d586Ddcf52439676Ca21A244753fA19F9Ea8e);
    IMeme constant hook = IMeme(0x223F8B1b43AEEc86d2659cCe36169f72f7FC2044);
    address constant implementation = 0x2D96E4e89D37551Ff55eB9cfEaD78DA1278D5986;
    uint160 constant Q96 = 79228162514264337593543950336;
    PoolKey key;
    uint16 tax;
    uint16 hookFee;
    string rows;
    bool nativePair;

    function setUp() public {
        vm.createSelectFork(vm.envString("TICKER_MEME_RPC"), 112361144);
        assertEq(implementation.codehash, 0x94a20f9305eeae246d3ec31488868fc17f81caf4a7ad2b05f528d04c55c4c382);
        vm.prank(hook.owner());
        hook.setFactory(address(this));
        vm.deal(address(this), 1e24);
    }

    function configure(bool native_, uint16 fee_, uint16 tax_) internal {
        nativePair = native_;
        hookFee = fee_;
        tax = tax_;
        FixtureToken a = new FixtureToken();
        FixtureToken b = new FixtureToken();
        address c0 = native_ ? address(0) : (address(a) < address(b) ? address(a) : address(b));
        address c1 = native_ ? address(a) : (address(a) < address(b) ? address(b) : address(a));
        a.mint(address(this), 1e24);
        b.mint(address(this), 1e24);
        key = PoolKey(Currency.wrap(c0), Currency.wrap(c1), 2000, 60, IHooks(address(hook)));
        manager.initialize(key, Q96);
        hook.registerPool(
            key, c1, address(this), address(this), tax_, true, FeePolicySnapshot(address(this), 3000, 5000, fee_, 300)
        );
        manager.unlock(abi.encode(uint8(0), true, int256(0)));
    }

    function unlockCallback(bytes calldata data) external returns (bytes memory) {
        require(msg.sender == address(manager));
        (uint8 op, bool direction, int256 specified) = abi.decode(data, (uint8, bool, int256));
        BalanceDelta delta;
        if (op == 0) {
            (delta,) = manager.modifyLiquidity(
                key, ModifyLiquidityParams(-887220, 887220, 1000000000000, bytes32(0)), ""
            );
        } else {
            delta = manager.swap(
                key,
                SwapParams(
                    direction,
                    specified,
                    direction ? uint160(4295128740) : uint160(1461446703485210103287273052203988822378723970341)
                ),
                ""
            );
        }
        settle(key.currency0, delta.amount0());
        settle(key.currency1, delta.amount1());
        return abi.encode(delta);
    }

    function settle(Currency currency, int128 amount) internal {
        if (amount > 0) {
            manager.take(currency, address(this), uint128(amount));
        } else if (amount < 0) {
            uint256 debt = uint256(-int256(amount));
            if (Currency.unwrap(currency) == address(0)) {
                manager.settle{value: debt}();
            } else {
                manager.sync(currency);
                FixtureToken(Currency.unwrap(currency)).transfer(address(manager), debt);
                manager.settle();
            }
        }
    }

    function trade(bool direction, uint256 input) internal {
        PoolId id = key.toId();
        (uint160 beforePrice, int24 beforeTick,,) = manager.getSlot0(id);
        address outputCurrency = Currency.unwrap(direction ? key.currency1 : key.currency0);
        uint256 feesBefore = hook.pendingFees(id, outputCurrency);
        uint256 taxBefore = hook.pendingCreatorTax(id, outputCurrency);
        uint256 buybackBefore = hook.pendingBuyback(id, outputCurrency);
        uint256 gasBefore = gasleft();
        BalanceDelta delta = abi.decode(manager.unlock(abi.encode(uint8(1), direction, -int256(input))), (BalanceDelta));
        uint256 used = gasBefore - gasleft();
        uint256 out = uint256(uint128(direction ? delta.amount1() : delta.amount0()));
        uint256 fee = hook.pendingFees(id, outputCurrency) - feesBefore;
        uint256 creatorTax = hook.pendingCreatorTax(id, outputCurrency) - taxBefore;
        uint256 gross = out + fee + creatorTax;
        assertEq(fee, gross * hookFee / 10000);
        assertEq(creatorTax, gross * tax / 10000);
        assertEq(hook.pendingBuyback(id, outputCurrency) - buybackBefore, (fee - fee * 3000 / 10000) * 5000 / 10000);
        assertEq(uint256(-int256(direction ? delta.amount0() : delta.amount1())), input);
        (uint160 afterPrice, int24 afterTick,,) = manager.getSlot0(id);
        rows = string.concat(
            rows,
            bytes(rows).length == 0 ? "" : ",",
            '{"zeroForOne":',
            direction ? "true" : "false",
            ',"input":"',
            vm.toString(input),
            '","grossOutput":"',
            vm.toString(gross),
            '","netOutput":"',
            vm.toString(out),
            '","hookFee":"',
            vm.toString(fee),
            '","creatorTax":"',
            vm.toString(creatorTax),
            '","beforeSqrtPriceX96":"',
            vm.toString(beforePrice),
            '","beforeTick":',
            vm.toString(int256(beforeTick)),
            ',"afterSqrtPriceX96":"',
            vm.toString(afterPrice),
            '","afterTick":',
            vm.toString(int256(afterTick)),
            ',"executionGas":',
            vm.toString(used),
            "}"
        );
    }

    function run(bool native_, uint16 fee_, uint16 tax_, string memory name) internal {
        configure(native_, fee_, tax_);
        trade(true, 1000000);
        trade(false, 2000000);
        trade(true, 100);
        trade(false, 1000000000);
        string memory document = string.concat(
            '{"chainId":143,"forkBlock":112361144,"native":',
            native_ ? "true" : "false",
            ',"hookFeeBps":',
            vm.toString(fee_),
            ',"creatorTaxBps":',
            vm.toString(tax_),
            ',"liquidity":"1000000000000","lpFee":2000,"tickSpacing":60,"trades":[',
            rows,
            "]}"
        );
        vm.writeFile(string.concat("./", name, ".json"), document);
    }

    function test_ERC20() public {
        run(false, 100, 133, "erc20");
    }

    function test_Native() public {
        run(true, 100, 133, "native");
    }

    function test_SeparateRounding() public {
        run(false, 100, 100, "rounding");
    }

    function test_ZeroFees() public {
        run(false, 0, 0, "zero-fees");
    }
    receive() external payable {}
}
